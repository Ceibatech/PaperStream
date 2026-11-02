package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	dir := t.TempDir()
	cfg = config{TokenURL: server.URL + "/token", ClientID: "mulcv-depot", ClientSecret: "s", APIBase: server.URL + "/api/v1",
		SentDir: filepath.Join(dir, "envoyes"), ErrorDir: filepath.Join(dir, "erreurs")}
	os.MkdirAll(cfg.SentDir, 0o750)
	os.MkdirAll(cfg.ErrorDir, 0o750)
	pattern = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*?)(?:_[0-9]{4,6})?$`)
	client = server.Client()
	token, retryDelay = "", time.Millisecond
	return dir
}

func scan(t *testing.T, dir, name string) string {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("%PDF-1.7 test"), 0o640); err != nil {
		t.Fatal(err)
	}
	return path
}

func tokenOr(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/token" {
		r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" {
			w.WriteHeader(400)
		}
		io.WriteString(w, `{"access_token":"jwt","expires_in":300}`)
		return true
	}
	return false
}

func TestTheDossierIdComesFromThePaperStreamFileName(t *testing.T) {
	var got string
	dir := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if tokenOr(w, r) {
			return
		}
		if r.URL.Path != "/api/v1/intake/scans" || r.Header.Get("Authorization") != "Bearer jwt" {
			w.WriteHeader(403)
			return
		}
		got = r.FormValue("dossierId")
		w.WriteHeader(201)
	})
	path := scan(t, dir, "DOS-000456_0001.pdf")
	if !process(path) || got != "DOS-000456" {
		t.Fatalf("dossier %q", got)
	}
	if _, err := os.Stat(filepath.Join(cfg.SentDir, "DOS-000456_0001.pdf")); err != nil {
		t.Fatal("le fichier envoyé doit être archivé")
	}
}

func TestARefusedScanIsSetAsideWithItsReason(t *testing.T) {
	dir := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if tokenOr(w, r) {
			return
		}
		w.WriteHeader(404)
		io.WriteString(w, `{"message":"Aucun dossier transféré ne porte cet identifiant"}`)
	})
	path := scan(t, dir, "DOS-9.pdf")
	if process(path) {
		t.Fatal("un refus ne doit pas être compté comme envoyé")
	}
	note, err := os.ReadFile(filepath.Join(cfg.ErrorDir, "DOS-9.pdf.txt"))
	if err != nil || !strings.Contains(string(note), "Aucun dossier") {
		t.Fatalf("raison absente : %q", note)
	}
}

func TestAnOutageKeepsTheFileForALaterTry(t *testing.T) {
	calls := 0
	dir := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if tokenOr(w, r) {
			return
		}
		calls++
		w.WriteHeader(502)
	})
	path := scan(t, dir, "DOS-1.pdf")
	if process(path) || calls != 4 {
		t.Fatalf("essais : %d", calls)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("le fichier doit rester en place")
	}
}

func TestAnyPaperStreamNameIsSentAndMulcvReadsTheQr(t *testing.T) {
	var hints []string
	var present []bool
	dir := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if tokenOr(w, r) {
			return
		}
		r.ParseMultipartForm(1 << 20)
		_, ok := r.MultipartForm.Value["dossierId"]
		present = append(present, ok)
		hints = append(hints, r.FormValue("dossierId"))
		if strings.Contains(r.MultipartForm.File["file"][0].Filename, "00001") {
			io.WriteString(w, `{"kind":"carton","cartonUid":"CG1"}`) // 200 : page du carton
			return
		}
		w.WriteHeader(201)
		io.WriteString(w, `{"kind":"dossier","cartonUid":"CG1","dossierId":"DOS-1","name":"DOS-1-x.pdf","identifiedBy":"qr"}`)
	})
	for _, name := range []string{"CCM_02112026_00001.pdf", "CCM 02-11 (2).pdf", "-scan.pdf"} {
		if !process(scan(t, dir, name)) {
			t.Fatalf("%s doit être accepté", name)
		}
		if _, err := os.Stat(filepath.Join(cfg.SentDir, name)); err != nil {
			t.Fatalf("%s doit être archivé", name)
		}
	}
	if hints[0] != "CCM_02112026" || present[1] || present[2] {
		t.Fatalf("indices envoyés : %q %v", hints, present)
	}
}

func TestAScanWithoutReadableQrIsSetAside(t *testing.T) {
	dir := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if tokenOr(w, r) {
			return
		}
		w.WriteHeader(422)
		io.WriteString(w, `{"message":"Aucun QR de dossier transféré lisible"}`)
	})
	if process(scan(t, dir, "0001.pdf")) {
		t.Fatal("un refus ne doit pas être compté comme envoyé")
	}
	if note, _ := os.ReadFile(filepath.Join(cfg.ErrorDir, "0001.pdf.txt")); !strings.Contains(string(note), "QR") {
		t.Fatalf("raison absente : %q", note)
	}
}

func TestTheClonedFoldersAreUsedByDefault(t *testing.T) {
	dir := t.TempDir()
	root = dir
	cfg = config{}
	defaults(&cfg.InboxDir, "numerisation", "1-a-envoyer")
	defaults(&cfg.SentDir, "numerisation", "2-envoyes")
	defaults(&cfg.LogFile, "journal", "mulcv-depot.log")
	if cfg.InboxDir != filepath.Join(dir, "numerisation", "1-a-envoyer") || cfg.SentDir != filepath.Join(dir, "numerisation", "2-envoyes") ||
		cfg.LogFile != filepath.Join(dir, "journal", "mulcv-depot.log") {
		t.Fatalf("emplacements : %+v", cfg)
	}
	cfg.SentDir = "D:\\autre"
	defaults(&cfg.SentDir, "numerisation", "2-envoyes")
	if cfg.SentDir != "D:\\autre" {
		t.Fatal("une valeur configurée ne doit pas être remplacée")
	}
}
