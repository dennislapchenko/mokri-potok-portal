// mokri-potok-portal backend. One binary: HTTP API + SQLite + nightly backup.
// `server healthcheck` probes the running instance (distroless has no curl).
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// Embeds the timezone database: the image is distroless and has none, and
	// notifications say "today" / "tomorrow" against the village's local date.
	// Set TZ=Europe/Ljubljana in the compose file to pick it up.
	_ "time/tzdata"

	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/config"
	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/httpapi"
	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/store"
)

func main() {
	cfg := config.Load()
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		resp, err := http.Get("http://127.0.0.1:" + cfg.Port + "/api/healthz")
		if err != nil || resp.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	srv := httpapi.New(st, cfg)

	// `/server code [name]` — the way back in when nobody is logged in. Run it
	// on the VM: SSH to the machine is the credential.
	if len(os.Args) > 1 && os.Args[1] == "code" {
		arg := ""
		if len(os.Args) > 2 {
			arg = os.Args[2]
		}
		if err := srv.Code(arg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if row, _ := st.One(context.Background(), `SELECT count(*) AS n FROM houses`); row != nil && row["n"].(int64) == 0 {
		code, err := srv.BootstrapCode()
		if err != nil {
			log.Fatalf("bootstrap code: %v", err)
		}
		// Printed once per boot while the village is empty. Read with docker logs.
		log.Printf("no houses yet — bootstrap the first steward house with code: %s", code)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go st.RunNightlyBackups(ctx)
	go srv.RunToolReminders(ctx)

	hs := &http.Server{Addr: ":" + cfg.Port, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sh, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(sh)
	}()
	log.Printf("listening on :%s, data in %s, timezone %s", cfg.Port, cfg.DataDir, time.Local)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
