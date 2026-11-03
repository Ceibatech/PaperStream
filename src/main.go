// mulcv-depot envoie les fichiers produits par PaperStream Capture vers MULCV.
//
// MULCV lit lui-même les QR collés sur les cartons et les dossiers : la page du carton
// ouvre le carton pour ce poste, le QR du dossier désigne le dossier où ranger le fichier.
// Aucun réglage de nom de fichier n'est nécessaire dans PaperStream ; si le nom est
// exactement un identifiant de dossier, il sert seulement de repli quand aucun QR n'est lisible.
// Les fichiers sont envoyés dans l'ordre de leur nom (ordre de numérisation).
//
// Arborescence attendue (le dépôt Git cloné dans C:\MULCV) :
//
//	C:\MULCV\bin\mulcv-depot.exe
//	C:\MULCV\config\mulcv-depot.json          écrit par installer.cmd
//	C:\MULCV\numerisation\1-a-envoyer\       sortie de PaperStream (mode dossier)
//	C:\MULCV\numerisation\2-envoyes\         fichiers déposés dans MULCV
//	C:\MULCV\numerisation\3-erreurs\         fichiers refusés + .txt de la raison
//	C:\MULCV\journal\mulcv-depot.log
//
//	mulcv-depot.exe <fichier> [fichiers...]   appelé par PaperStream « Other App » avec %f
//	mulcv-depot.exe -watch                    surveille numerisation\1-a-envoyer
//	mulcv-depot.exe -test                     vérifie la configuration et la connexion
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type config struct {
	TokenURL     string `json:"tokenUrl"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	APIBase      string `json:"apiBase"`
	NamePattern  string `json:"namePattern"`
	SentDir      string `json:"sentDir"`
	ErrorDir     string `json:"errorDir"`
	InboxDir     string `json:"inboxDir"`
	LogFile      string `json:"logFile"`
}

const maxBytes = 25 << 20

var (
	cfg     config
	pattern *regexp.Regexp
	client  = &http.Client{Timeout: 2 * time.Minute}
	token   string
	expiry  time.Time
	// Attente entre deux essais, multipliée par le carré du numéro d'essai.
	retryDelay = 5 * time.Second
)

func main() {
	configPath := flag.String("config", "", "chemin du fichier de configuration (défaut : mulcv-depot.json à côté du programme)")
	watch := flag.Bool("watch", false, "surveiller numerisation\\1-a-envoyer")
	test := flag.Bool("test", false, "vérifier la configuration et la connexion")
	flag.Parse()

	if err := load(*configPath); err != nil {
		fail("Configuration : %v", err)
	}
	switch {
	case *test:
		if _, err := accessToken(); err != nil {
			fail("Connexion refusée : %v", err)
		}
		log.Printf("Configuration correcte, connexion à MULCV établie.")
	case *watch:
		watchDir(cfg.InboxDir)
	default:
		if flag.NArg() == 0 {
			fail("Indiquez un fichier, -watch <dossier> ou -test.")
		}
		// Un seul envoi à la fois sur le poste : les fichiers d'un même dossier partent dans
		// l'ordre où PaperStream les a produits (MULCV rattache une suite sans QR au dossier précédent).
		unlock := lock()
		defer unlock()
		failed := 0
		for _, path := range flag.Args() {
			if !process(path) {
				failed++
			}
		}
		if failed > 0 {
			unlock()
			os.Exit(1)
		}
	}
}

// root est le dossier qui contient bin\ : C:\MULCV quand le dépôt y est cloné.
var root string

func load(path string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	root = filepath.Dir(filepath.Dir(exe))
	if path == "" {
		path = filepath.Join(root, "config", "mulcv-depot.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("%s : %w", path, err)
	}
	if cfg.NamePattern == "" {
		// Nom du fichier = identifiant du dossier, éventuellement suivi d'un compteur PaperStream (_0001).
		cfg.NamePattern = `^([A-Za-z0-9][A-Za-z0-9._-]*?)(?:_[0-9]{4,6})?$`
	}
	if pattern, err = regexp.Compile(cfg.NamePattern); err != nil {
		return fmt.Errorf("namePattern : %w", err)
	}
	for name, value := range map[string]string{"tokenUrl": cfg.TokenURL, "apiBase": cfg.APIBase} {
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("%s doit être une adresse https", name)
		}
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return errors.New("clientId et clientSecret sont obligatoires")
	}
	// Emplacements par défaut : ceux du dépôt cloné, déjà présents.
	defaults(&cfg.InboxDir, "numerisation", "1-a-envoyer")
	defaults(&cfg.SentDir, "numerisation", "2-envoyes")
	defaults(&cfg.ErrorDir, "numerisation", "3-erreurs")
	defaults(&cfg.LogFile, "journal", "mulcv-depot.log")
	for _, dir := range []string{cfg.InboxDir, cfg.SentDir, cfg.ErrorDir, filepath.Dir(cfg.LogFile)} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}
	{
		file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			return err
		}
		log.SetOutput(io.MultiWriter(os.Stderr, file))
	}
	return nil
}

func defaults(value *string, parts ...string) {
	if *value == "" {
		*value = filepath.Join(append([]string{root}, parts...)...)
	}
}

func watchDir(dir string) {
	log.Printf("Surveillance de %s", dir)
	sizes := map[string]int64{}
	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			log.Printf("Lecture de %s impossible : %v", dir, err)
		}
		seen := map[string]bool{}
		for _, entry := range entries {
			if entry.IsDir() || !accepted(entry.Name()) {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}
			seen[path] = true
			// Un fichier n'est envoyé que lorsque sa taille ne bouge plus entre deux passages.
			if previous, ok := sizes[path]; ok && previous == info.Size() && info.Size() > 0 {
				process(path)
				delete(sizes, path)
				continue
			}
			sizes[path] = info.Size()
		}
		for path := range sizes {
			if !seen[path] {
				delete(sizes, path)
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func accepted(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf", ".jpg", ".jpeg", ".png":
		return true
	}
	return false
}

// process renvoie true si le fichier a été déposé dans MULCV.
func process(path string) bool {
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	// Indice facultatif : le nom sans le compteur PaperStream (_0001). MULCV l'ignore s'il
	// ne correspond pas exactement à un dossier transféré.
	hint := ""
	if match := pattern.FindStringSubmatch(stem); len(match) > 1 {
		hint = match[1]
	}
	info, err := os.Stat(path)
	if err != nil {
		log.Printf("%s : %v", base, err)
		return false
	}
	if info.Size() == 0 || info.Size() > maxBytes {
		return reject(path, "fichier vide ou supérieur à 25 Mo")
	}
	var last error
	for attempt := 1; attempt <= 4; attempt++ {
		status, body, err := send(path, hint)
		switch {
		case err == nil && (status == http.StatusCreated || status == http.StatusOK):
			log.Printf("%s : %s", base, outcome(body))
			return move(path, cfg.SentDir)
		case err == nil && status == http.StatusUnauthorized:
			token = "" // jeton expiré : on en redemande un
			last = errors.New("session expirée")
		case err == nil && status >= 400 && status < 500:
			return reject(path, fmt.Sprintf("refusé par MULCV (%d) : %s", status, message(body)))
		case err != nil:
			last = err
		default:
			last = fmt.Errorf("MULCV indisponible (%d)", status)
		}
		time.Sleep(time.Duration(attempt*attempt) * retryDelay)
	}
	// Erreur temporaire : le fichier reste en place pour un nouvel essai.
	log.Printf("%s non envoyé, nouvel essai plus tard : %v", base, last)
	return false
}

// lock attend que l'envoi précédent du poste soit fini. Un verrou de plus de 10 minutes
// (programme interrompu) est considéré comme abandonné.
func lock() func() {
	path := filepath.Join(filepath.Dir(cfg.LogFile), "envoi.lock")
	for start := time.Now(); time.Since(start) < 15*time.Minute; time.Sleep(200 * time.Millisecond) {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
		if err == nil {
			file.Close()
			var once bool
			return func() {
				if !once {
					once = true
					os.Remove(path)
				}
			}
		}
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > 10*time.Minute {
			os.Remove(path)
		}
	}
	log.Printf("Verrou d'envoi toujours pris après 15 minutes : envoi sans attendre")
	return func() {}
}

// outcome résume la réponse de MULCV pour le journal : un dossier par ligne.
func outcome(body []byte) string {
	type document struct {
		DossierID    string `json:"dossierId"`
		CartonUID    string `json:"cartonUid"`
		Name         string `json:"name"`
		IdentifiedBy string `json:"identifiedBy"`
		Pages        string `json:"pages"`
	}
	var r struct {
		Kind      string     `json:"kind"`
		CartonUID string     `json:"cartonUid"`
		Documents []document `json:"documents"`
		document
	}
	if json.Unmarshal(body, &r) != nil {
		return "accepté par MULCV"
	}
	if r.Kind == "carton" {
		return "page du carton " + r.CartonUID + " : carton ouvert, les dossiers suivants y seront rangés"
	}
	if len(r.Documents) == 0 && r.DossierID != "" {
		r.Documents = []document{r.document}
	}
	lines := []string{fmt.Sprintf("%d dossier(s) déposé(s)", len(r.Documents))}
	for _, d := range r.Documents {
		how := "QR"
		switch d.IdentifiedBy {
		case "nom":
			how = "nom du fichier"
		case "suite":
			how = "suite du dossier précédent"
		}
		pages := ""
		if d.Pages != "" {
			pages = ", pages " + d.Pages
		}
		lines = append(lines, fmt.Sprintf("  - %s (carton %s, %s%s) sous %s", d.DossierID, d.CartonUID, how, pages, d.Name))
	}
	return strings.Join(lines, "\n")
}

func send(path, dossierID string) (int, []byte, error) {
	bearer, err := accessToken()
	if err != nil {
		return 0, nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, nil, err
	}
	defer file.Close()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if dossierID != "" {
		if err := form.WriteField("dossierId", dossierID); err != nil {
			return 0, nil, err
		}
	}
	part, err := form.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return 0, nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return 0, nil, err
	}
	form.Close()
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(cfg.APIBase, "/")+"/intake/scans", &body)
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+bearer)
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	return response.StatusCode, data, nil
}

func accessToken() (string, error) {
	if token != "" && time.Now().Before(expiry) {
		return token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {cfg.ClientID}, "client_secret": {cfg.ClientSecret}}
	response, err := client.PostForm(cfg.TokenURL, form)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Keycloak a répondu %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil || result.AccessToken == "" {
		return "", errors.New("réponse Keycloak inattendue")
	}
	token = result.AccessToken
	expiry = time.Now().Add(time.Duration(max(result.ExpiresIn-30, 10)) * time.Second)
	return token, nil
}

func message(body []byte) string {
	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil {
		if payload.Message != "" {
			return payload.Message
		}
		if payload.Error != "" {
			return payload.Error
		}
	}
	return "sans détail"
}

// reject range le fichier dans errorDir avec un .txt expliquant pourquoi.
func reject(path, reason string) bool {
	log.Printf("%s : %s", filepath.Base(path), reason)
	if move(path, cfg.ErrorDir) {
		note := filepath.Join(cfg.ErrorDir, filepath.Base(path)+".txt")
		_ = os.WriteFile(note, []byte(time.Now().Format(time.RFC3339)+" "+reason+"\r\n"), 0o640)
	}
	return false
}

func move(path, dir string) bool {
	target := filepath.Join(dir, filepath.Base(path))
	if _, err := os.Stat(target); err == nil {
		ext := filepath.Ext(target)
		target = strings.TrimSuffix(target, ext) + "-" + time.Now().Format("20060102-150405") + ext
	}
	if err := os.Rename(path, target); err != nil {
		// Volumes différents : copie puis suppression.
		if copyErr := copyFile(path, target); copyErr != nil {
			log.Printf("Impossible de déplacer %s : %v", path, copyErr)
			return false
		}
		_ = os.Remove(path)
	}
	return true
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func fail(format string, args ...any) {
	log.Printf(format, args...)
	os.Exit(2)
}
