package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSiteFiles: a village's own backdrop is a row that serves in the built-in
// file's place, public like the gate it stands behind, with an ETag; only an
// image is taken; and the shell carries the village's first language.
func TestSiteFiles(t *testing.T) {
	srv, _, _, _ := newVillage(t)
	if err := srv.ImportSite("backdrop", strings.NewReader("not a picture")); err == nil {
		t.Fatal("text was taken as a backdrop")
	}
	if err := srv.ImportSite("icon", strings.NewReader("\x89PNG\r\n\x1a\n")); err == nil {
		t.Fatal("an unknown name was taken")
	}
	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 64)
	if err := srv.ImportSite("backdrop", strings.NewReader(png)); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/backdrop.jpg", nil))
	etag := rec.Header().Get("ETag")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" || rec.Body.String() != png || etag == "" {
		t.Fatalf("backdrop: %d %q %d bytes", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
	}
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/backdrop.jpg", nil)
	req.Header.Set("If-None-Match", etag)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 304 {
		t.Fatalf("a phone that holds the backdrop fetched it again: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(rec.Body.String(), `<html lang="sl">`) {
		t.Fatalf("the shell without the village's language: %q", rec.Body.String())
	}
}
