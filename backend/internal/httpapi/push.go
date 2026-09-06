package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Web push. The VAPID key pair identifies this backend to the browsers' push
// services; it is generated once and kept in the settings table, so it lives
// in the same SQLite file as everything else and travels with the backups.

var Kinds = []string{"posts", "needs", "offers", "runs", "events", "away", "tools", "projects", "camp"}

// Sender is the one seam for tests: production sends over the network.
type Sender interface {
	Send(ctx context.Context, sub Subscription, payload []byte) (status int, err error)
}

type Subscription struct {
	ID       int64
	Endpoint string
	P256dh   string
	Auth     string
	Lang     string
}

type Payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Kind  string `json:"kind"`
}

type webpushSender struct{ pub, priv, subject string }

func (w webpushSender) Send(ctx context.Context, sub Subscription, payload []byte) (int, error) {
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth}},
		&webpush.Options{Subscriber: w.subject, VAPIDPublicKey: w.pub, VAPIDPrivateKey: w.priv, TTL: 6 * 3600})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// vapid returns the stored key pair, generating it on first use.
func (s *Server) vapid() (pub, priv string, err error) {
	ctx := context.Background()
	row, err := s.st.One(ctx, `SELECT value FROM settings WHERE key='vapid'`)
	if err != nil {
		return "", "", err
	}
	if row != nil {
		parts := strings.SplitN(row["value"].(string), " ", 2)
		return parts[0], parts[1], nil
	}
	priv, pub, err = webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", err
	}
	_, err = s.st.Exec(ctx, `INSERT INTO settings(key,value) VALUES ('vapid', ?)`, pub+" "+priv)
	return pub, priv, err
}

func (s *Server) sender() (Sender, error) {
	if s.send != nil {
		return s.send, nil
	}
	pub, priv, err := s.vapid()
	if err != nil {
		return nil, err
	}
	s.send = webpushSender{pub: pub, priv: priv, subject: s.cfg.PushSubject}
	return s.send, nil
}

// fanout renders the text once per language and sends it to the phones the
// query selects. A dead subscription (404/410) is deleted. Runs detached.
func (s *Server) fanout(kind string, build func(lang string) Payload, query string, args ...any) {
	rendered := map[string][]byte{}
	body := func(lang string) []byte {
		if b, ok := rendered[lang]; ok {
			return b
		}
		p := build(lang)
		p.Kind = kind
		b, _ := json.Marshal(p)
		rendered[lang] = b
		return b
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rows, err := s.st.Rows(ctx, query, args...)
		if err != nil {
			log.Printf("push: recipients: %v", err)
			return
		}
		if len(rows) == 0 {
			return
		}
		snd, err := s.sender()
		if err != nil {
			log.Printf("push: sender: %v", err)
			return
		}
		for _, r := range rows {
			sub := Subscription{ID: r["id"].(int64), Endpoint: r["endpoint"].(string), P256dh: r["p256dh"].(string), Auth: r["auth"].(string), Lang: r["lang"].(string)}
			status, err := snd.Send(ctx, sub, body(sub.Lang))
			if err != nil {
				log.Printf("push: send %d: %v", sub.ID, err)
				continue
			}
			if status == http.StatusNotFound || status == http.StatusGone {
				s.st.Exec(ctx, `DELETE FROM push_subscriptions WHERE id=?`, sub.ID)
			}
		}
	}()
}

// subsFor takes the kind twice: once for the house's own off-list, once for the
// steward's village-wide one.
const subsFor = `SELECT p.id, p.endpoint, p.p256dh, p.auth, p.lang FROM push_subscriptions p
	WHERE NOT EXISTS (SELECT 1 FROM notify_off n WHERE n.house_id=p.house_id AND n.kind=?)
	AND NOT EXISTS (SELECT 1 FROM notify_off_global g WHERE g.kind=?) `

// quietHours: between 21:00 and 07:00 only an alarm is worth a buzz. The item
// still waits in the app — a village that stops trusting its phone at night
// turns notifications off altogether.
func (s *Server) quietHours() bool {
	h := s.now().Hour()
	return h >= 21 || h < 7
}

// notify tells every house except the one that caused the thing.
func (s *Server) notify(kind string, fromHouse int64, build func(lang string) Payload) {
	if s.quietHours() {
		return
	}
	s.fanout(kind, build, subsFor+`AND p.house_id != ?`, kind, kind, fromHouse)
}

// notifyHouse sends to one house only — used when the message is that house's
// business alone: someone signed up to its work bee, or borrowed its tool.
func (s *Server) notifyHouse(kind string, toHouse int64, build func(lang string) Payload) {
	if s.quietHours() {
		return
	}
	s.fanout(kind, build, subsFor+`AND p.house_id = ?`, kind, kind, toHouse)
}

// ---- handlers ------------------------------------------------------------

func (s *Server) pushKey(w http.ResponseWriter, r *http.Request) {
	pub, _, err := s.vapid()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"key": pub})
}

// subscribe stores this phone's push subscription (idempotent on endpoint).
func (s *Server) subscribe(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	keys, _ := m["keys"].(map[string]any)
	endpoint := str(m, "endpoint")
	p256, _ := keys["p256dh"].(string)
	auth, _ := keys["auth"].(string)
	if endpoint == "" || p256 == "" || auth == "" {
		writeErr(w, 400, "endpoint and keys required")
		return
	}
	lang := str(m, "lang")
	if lang != "en" {
		lang = "sl"
	}
	if _, err := s.st.Exec(r.Context(), `INSERT INTO push_subscriptions(house_id, device_id, endpoint, p256dh, auth, lang) VALUES (?,?,?,?,?,?)
		ON CONFLICT(endpoint) DO UPDATE SET house_id=excluded.house_id, device_id=excluded.device_id, p256dh=excluded.p256dh, auth=excluded.auth, lang=excluded.lang`,
		h.ID, h.DeviceID, endpoint, p256, auth, lang); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) unsubscribe(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	s.st.Exec(r.Context(), `DELETE FROM push_subscriptions WHERE endpoint=? AND house_id=?`, str(m, "endpoint"), houseFrom(r).ID)
	w.WriteHeader(204)
}

// prefs: GET returns {off: [...kinds switched off]}, PUT replaces that list.
func (s *Server) getPrefs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT kind FROM notify_off WHERE house_id=?`, houseFrom(r).ID)
	if err != nil {
		fail(w, err)
		return
	}
	off := []string{}
	for _, x := range rows {
		off = append(off, x["kind"].(string))
	}
	subs, _ := s.st.One(r.Context(), `SELECT count(*) AS n FROM push_subscriptions WHERE house_id=?`, houseFrom(r).ID)
	grows, _ := s.st.Rows(r.Context(), `SELECT g.kind, g.set_at, h.name AS set_by FROM notify_off_global g LEFT JOIN houses h ON h.id=g.set_by`)
	goff := []string{}
	for _, x := range grows {
		goff = append(goff, x["kind"].(string))
	}
	writeJSON(w, 200, map[string]any{"off": off, "global_off": goff, "global_detail": grows, "kinds": Kinds, "phones": subs["n"]})
}

// putGlobalPrefs: a steward mutes kinds for every house at once — the lever for
// "the village finds needs too noisy" without asking each phone.
func (s *Server) putGlobalPrefs(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	off, _ := m["off"].([]any)
	want := map[string]bool{}
	for _, k := range off {
		if ks, ok := k.(string); ok && contains(Kinds, ks) {
			want[ks] = true
		}
	}
	// Keep existing rows (their who/when stays true); add new ones stamped with
	// this steward; drop the ones switched back on.
	h := houseFrom(r)
	for _, k := range Kinds {
		if want[k] {
			s.st.Exec(r.Context(), `INSERT OR IGNORE INTO notify_off_global(kind, set_by) VALUES (?,?)`, k, h.ID)
		} else {
			s.st.Exec(r.Context(), `DELETE FROM notify_off_global WHERE kind=?`, k)
		}
	}
	log.Printf("global mute by house %d: %v", h.ID, off)
	w.WriteHeader(204)
}

func (s *Server) putPrefs(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	off, _ := m["off"].([]any)
	s.st.Exec(r.Context(), `DELETE FROM notify_off WHERE house_id=?`, h.ID)
	for _, k := range off {
		if ks, ok := k.(string); ok && contains(Kinds, ks) {
			s.st.Exec(r.Context(), `INSERT OR IGNORE INTO notify_off(house_id, kind) VALUES (?,?)`, h.ID, ks)
		}
	}
	w.WriteHeader(204)
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// snippet trims a body for a notification line.
func snippet(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
