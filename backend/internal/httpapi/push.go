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

var Kinds = []string{"posts", "needs", "offers", "runs", "events", "away"}

// Sender is the one seam for tests: production sends over the network.
type Sender interface {
	Send(ctx context.Context, sub Subscription, payload []byte) (status int, err error)
}

type Subscription struct {
	ID       int64
	Endpoint string
	P256dh   string
	Auth     string
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

// notify fans a payload out to every phone of every house that did not switch
// this kind off, except the house that caused it. Runs in the background; a
// dead subscription (404/410) is deleted.
func (s *Server) notify(kind string, fromHouse int64, p Payload) {
	p.Kind = kind
	body, _ := json.Marshal(p)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		subs, err := s.recipients(ctx, kind, fromHouse)
		if err != nil {
			log.Printf("push: recipients: %v", err)
			return
		}
		if len(subs) == 0 {
			return
		}
		snd, err := s.sender()
		if err != nil {
			log.Printf("push: sender: %v", err)
			return
		}
		for _, sub := range subs {
			status, err := snd.Send(ctx, sub, body)
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

func (s *Server) recipients(ctx context.Context, kind string, fromHouse int64) ([]Subscription, error) {
	rows, err := s.st.Rows(ctx, `SELECT p.id, p.endpoint, p.p256dh, p.auth FROM push_subscriptions p
		WHERE p.house_id != ? AND NOT EXISTS (SELECT 1 FROM notify_off n WHERE n.house_id=p.house_id AND n.kind=?)`, fromHouse, kind)
	if err != nil {
		return nil, err
	}
	out := make([]Subscription, 0, len(rows))
	for _, r := range rows {
		out = append(out, Subscription{ID: r["id"].(int64), Endpoint: r["endpoint"].(string), P256dh: r["p256dh"].(string), Auth: r["auth"].(string)})
	}
	return out, nil
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
	if _, err := s.st.Exec(r.Context(), `INSERT INTO push_subscriptions(house_id, device_id, endpoint, p256dh, auth) VALUES (?,?,?,?,?)
		ON CONFLICT(endpoint) DO UPDATE SET house_id=excluded.house_id, device_id=excluded.device_id, p256dh=excluded.p256dh, auth=excluded.auth`,
		h.ID, h.DeviceID, endpoint, p256, auth); err != nil {
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
	writeJSON(w, 200, map[string]any{"off": off, "kinds": Kinds, "phones": subs["n"]})
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
