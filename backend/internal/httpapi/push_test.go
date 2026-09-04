package httpapi

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/config"
	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/store"
)

type fakeSender struct {
	mu       sync.Mutex
	sent     []Subscription
	payloads [][]byte
	status   map[string]int // endpoint -> status to return
}

func (f *fakeSender) Send(_ context.Context, sub Subscription, payload []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sub)
	f.payloads = append(f.payloads, payload)
	if st, ok := f.status[sub.Endpoint]; ok {
		return st, nil
	}
	return 201, nil
}

func (f *fakeSender) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.sent) }

func waitFor(t *testing.T, want int, f *fakeSender) {
	deadline := time.Now().Add(3 * time.Second)
	for f.count() < want && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // catch over-sends
	if f.count() != want {
		t.Fatalf("want %d sends got %d", want, f.count())
	}
}

// TestPushFanout: three houses, two subscribed; author never gets its own
// post; a house that switched posts off gets nothing; a 410 endpoint is dropped.
func TestPushFanout(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	fake := &fakeSender{status: map[string]int{}}
	srv := New(st, config.Config{BootstrapCode: "x"})
	srv.send = fake
	srv.now = func() time.Time { return time.Date(2026, 9, 4, 10, 0, 0, 0, time.Local) } // outside quiet hours
	h := srv.Handler()

	steward := &client{t: t, h: h}
	_, obj, _ := steward.do("POST", "/api/bootstrap", map[string]any{"code": "x", "name": "S"})
	steward.token = obj["token"].(string)
	mk := func(name string) *client {
		_, o, _ := steward.do("POST", "/api/houses", map[string]any{"name": name})
		c := &client{t: t, h: h}
		_, j, _ := c.do("POST", "/api/join", map[string]any{"code": o["invite"].(map[string]any)["code"]})
		c.token = j["token"].(string)
		return c
	}
	a, b := mk("A"), mk("B")
	sub := func(c *client, ep string) {
		code, _, _ := c.do("POST", "/api/push/subscribe", map[string]any{"endpoint": ep, "keys": map[string]any{"p256dh": "p", "auth": "a"}})
		c.must(204, code, "subscribe "+ep)
	}
	sub(a, "https://push/a1")
	sub(a, "https://push/a2")
	sub(b, "https://push/b1")

	code, _, _ := steward.do("GET", "/api/push/key", nil)
	steward.must(200, code, "vapid key")

	// Steward posts: A (2 phones) and B (1 phone) get it, steward does not.
	steward.do("POST", "/api/posts", map[string]any{"body": "hello"})
	waitFor(t, 3, fake)

	// B switches posts off; A posts: only... nobody except steward has no phones -> 0 sends.
	code, _, _ = b.do("PUT", "/api/me/prefs", map[string]any{"off": []string{"posts"}})
	b.must(204, code, "prefs")
	_, prefs, _ := b.do("GET", "/api/me/prefs", nil)
	if off := prefs["off"].([]any); len(off) != 1 || off[0] != "posts" {
		t.Fatalf("prefs: %v", prefs)
	}
	fake.mu.Lock()
	fake.sent = nil
	fake.mu.Unlock()
	a.do("POST", "/api/posts", map[string]any{"body": "from A"})
	waitFor(t, 0, fake)

	// A need from A still reaches B (only posts are off). B's endpoint is dead -> dropped.
	fake.status["https://push/b1"] = 410
	a.do("POST", "/api/needs", map[string]any{"text": "salt"})
	waitFor(t, 1, fake)
	time.Sleep(50 * time.Millisecond)
	_, p2, _ := b.do("GET", "/api/me/prefs", nil)
	if p2["phones"].(float64) != 0 {
		t.Fatalf("dead subscription not dropped: %v", p2)
	}
}
