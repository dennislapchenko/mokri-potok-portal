package httpapi

import (
	"context"
	"log"
	"time"
)

// Tool reminders. A tool out for more than a day earns its holder one push a
// day: "you still have the chainsaw". Only the holder hears it, never the
// owner, and nothing is counted — it is a nudge, not a ledger. Quiet hours are
// skipped entirely (not sent-and-dropped), so the nudge lands in the morning.

// RunToolReminders blocks and checks every 30 minutes.
func (s *Server) RunToolReminders(ctx context.Context) {
	t := time.NewTicker(30 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.quietHours() {
				continue
			}
			if n, err := s.remindTools(ctx); err != nil {
				log.Printf("tool reminders: %v", err)
			} else if n > 0 {
				log.Printf("tool reminders: %d sent", n)
			}
		}
	}
}

// remindTools nudges the holder of each overdue tool and stamps reminded_at.
// The gaps double — after 1 day, then 2 more, then 4, then 8 — so a job that
// takes a week costs the borrower two buzzes, not seven. No counter is kept:
// the next gap is read off the two timestamps.
func (s *Server) remindTools(ctx context.Context) (int, error) {
	rows, err := s.st.Rows(ctx, `SELECT t.id, t.name, t.held_by, t.held_since, t.reminded_at, o.name AS owner
		FROM tools t JOIN houses o ON o.id=t.house_id WHERE t.held_by IS NOT NULL`)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	sent := 0
	for _, r := range rows {
		since, ok := parseSQLiteTime(r["held_since"])
		if !ok || now.Sub(since) < 24*time.Hour {
			continue
		}
		if last, ok := parseSQLiteTime(r["reminded_at"]); ok {
			gap := last.Sub(since)
			if gap < 24*time.Hour {
				gap = 24 * time.Hour
			}
			if now.Sub(last) < gap {
				continue
			}
		}
		name, owner := r["name"].(string), r["owner"].(string)
		s.notifyHouse("tools", r["held_by"].(int64), func(lang string) Payload {
			return Payload{
				Title: tr(lang, "🛠 Še imaš: ", "🛠 You still have: ") + name,
				Body:  tr(lang, "od ", "from ") + owner + tr(lang, " — vrni ali označi vrnjeno", " — return it or mark it returned"),
				URL:   "#/shed",
			}
		})
		if _, err := s.st.Exec(ctx, `UPDATE tools SET reminded_at=datetime('now') WHERE id=?`, r["id"]); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

// parseSQLiteTime reads the two shapes SQLite writes: a date, or a datetime.
func parseSQLiteTime(v any) (time.Time, bool) {
	str, ok := v.(string)
	if !ok || str == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, str); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
