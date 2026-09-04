package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/store"
)

// House is the identity attached to every authenticated request. All members
// of a house share it; the device row tells them apart.
type House struct {
	ID        int64
	Name      string
	IsSteward bool
	DeviceID  int64
}

type ctxKey struct{}

func houseFrom(r *http.Request) *House {
	h, _ := r.Context().Value(ctxKey{}).(*House)
	return h
}

func randomToken(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// inviteCode is short and phone-typable: 10 chars, no ambiguous letters.
func inviteCode() string {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	b := make([]byte, 10)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// pairingCode is six digits: short enough to read across a kitchen table,
// short-lived and single-use to make up for the small keyspace.
func pairingCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	out := make([]byte, 6)
	for i := range b {
		out[i] = '0' + b[i]%10
	}
	return string(out)
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// requireHouse resolves the bearer token to a house or answers 401.
func (s *Server) requireHouse(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "no token")
			return
		}
		row, err := s.st.One(r.Context(), `SELECT d.id AS device_id, h.id, h.name, h.is_steward
			FROM devices d JOIN houses h ON h.id=d.house_id WHERE d.token_hash=?`, hashToken(strings.TrimPrefix(auth, "Bearer ")))
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		if row == nil {
			writeErr(w, http.StatusUnauthorized, "unknown device")
			return
		}
		h := &House{ID: row["id"].(int64), Name: row["name"].(string), IsSteward: row["is_steward"].(int64) == 1, DeviceID: row["device_id"].(int64)}
		go s.st.Exec(context.Background(), `UPDATE devices SET last_seen=datetime('now') WHERE id=?`, h.DeviceID)
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, h)))
	}
}

func (s *Server) requireSteward(next http.HandlerFunc) http.HandlerFunc {
	return s.requireHouse(func(w http.ResponseWriter, r *http.Request) {
		if !houseFrom(r).IsSteward {
			writeErr(w, http.StatusForbidden, "stewards only")
			return
		}
		next(w, r)
	})
}

// newDevice mints a token for a house and stores its hash.
func (s *Server) newDevice(ctx context.Context, houseID int64, label string) (string, error) {
	tok := randomToken(32)
	_, err := s.st.Exec(ctx, `INSERT INTO devices(house_id, token_hash, label) VALUES (?,?,?)`, houseID, hashToken(tok), label)
	return tok, err
}

func (s *Server) newInvite(ctx context.Context, houseID int64) (store.Row, error) {
	if _, err := s.st.Exec(ctx, `DELETE FROM invites WHERE house_id=?`, houseID); err != nil {
		return nil, err
	}
	code := inviteCode()
	exp := time.Now().UTC().Add(14 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := s.st.Exec(ctx, `INSERT INTO invites(code, house_id, expires_at) VALUES (?,?,?)`, code, houseID, exp); err != nil {
		return nil, err
	}
	return store.Row{"code": code, "expires_at": exp}, nil
}
