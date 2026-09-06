package httpapi

import (
	"context"
	"fmt"
	"strings"
)

// The way back in when nobody is logged in on a device — a steward who lost the
// session, or a house that never opened the link. Run on the VM over SSH, so
// the credential to use it is SSH access to the machine, which is the strongest
// one the village has:
//
//	docker exec mokri-potok-potok-api-1 /server code "Solnce"
//
// It rotates that house's invite and prints the link. The old link stops
// working, which is the point: an invite that leaks is replaced, not shared.

// Code prints a fresh invite link for the house whose name contains `match`.
// With an empty match it lists the houses.
func (s *Server) Code(match string) error {
	ctx := context.Background()
	rows, err := s.st.Rows(ctx, `SELECT id, name, crest, kind, is_steward FROM houses ORDER BY name`)
	if err != nil {
		return err
	}
	if match == "" {
		fmt.Println("houses:")
		for _, r := range rows {
			star := ""
			if r["is_steward"].(int64) == 1 {
				star = "  (steward)"
			}
			fmt.Printf("  %-30s %s%s\n", r["name"], r["kind"], star)
		}
		fmt.Println("\nrun again with part of a name, e.g. /server code \"Solnce\"")
		return nil
	}
	var hit map[string]any
	for _, r := range rows {
		if r["kind"] == "common" {
			continue // land, not an account
		}
		if strings.Contains(strings.ToLower(r["name"].(string)), strings.ToLower(match)) {
			if hit != nil {
				return fmt.Errorf("%q matches more than one house", match)
			}
			hit = r
		}
	}
	if hit == nil {
		return fmt.Errorf("no house matches %q", match)
	}
	inv, err := s.newInvite(ctx, hit["id"].(int64))
	if err != nil {
		return err
	}
	base := strings.TrimSuffix(s.cfg.PublicURL, "/")
	fmt.Printf("\n  house : %s %s\n  code  : %s\n  link  : %s/#/join/%s\n  until : %s\n\n",
		hit["crest"], hit["name"], inv["code"], base, inv["code"], inv["expires_at"])
	fmt.Println("  Any earlier link for this house has stopped working.")
	return nil
}
