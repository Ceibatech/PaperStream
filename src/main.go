// mulcv-depot envoie les fichiers produits par PaperStream Capture vers MULCV.
//
// Chaque fichier porte en nom l'identifiant GeoArchiv du dossier (valeur du QR lue
// par PaperStream). Le backend MULCV retrouve ce dossier et y dépose le fichier ;
// le service OCR du dossier Scan prend ensuite le relais.
//
//	mulcv-depot.exe <fichier> [fichiers...]   appelé par PaperStream « Autre application » avec %f
//	mulcv-depot.exe -watch <dossier>          surveille un dossier de sortie PaperStream
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
	watch := flag.String("watch", "", "dossier à surveiller")
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
	case *watch != "":
		watchDir(*watch)
	default:
		if flag.NArg() == 0 {
			fail("Indiquez un fichier, -watch <dossier> ou -test.")
		}
		failed := 0
		for _, path := range flag.Args() {
			if !process(path) {
				failed++
			}
		}
		if failed > 0 {
			os.Exit(1)
		}
	}
}

func load(path string) error {
	if path == "" {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		path = filepath.Join(filepath.Dir(exe), "mulcv-depot.json")
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
	for _, dir := range []string{cfg.SentDir, cfg.ErrorDir} {
		if dir == "" {
			return errors.New("sentDir et errorDir sont obligatoires")
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}
	if cfg.LogFile != "" {
		file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			return err
		}
		log.SetOutput(io.MultiWriter(os.Stderr, file))
	}
	return nil
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
	match := pattern.FindStringSubmatch(stem)
	if match == nil || len(match) < 2 || match[1] == "" {
		return reject(path, "le nom du fichier ne contient pas d'identifiant de dossier")
	}
	dossierID := match[1]
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
		status, body, err := send(path, dossierID)
		switch {
		case err == nil && status == http.StatusCreated:
			log.Printf("%s déposé dans le dossier %s", base, dossierID)
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
	if err := form.WriteField("dossierId", dossierID); err != nil {
		return 0, nil, err
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
