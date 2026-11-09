// Command leads accepts demo requests from the landing (POST /api/leads).
//
// Data flow (152-FZ): the lead is first written to a local file on the server in Russia
// (primary record + consent evidence), then emailed to the team mailbox (hosted in Russia).
// Telegram gets a notification WITHOUT personal data: it is a foreign service.
package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type config struct {
	addr     string
	dataDir  string
	smtpHost string
	smtpPort string
	smtpUser string
	smtpPass string
	mailTo   string
	tgToken  string
	tgChat   string
}

func loadConfig() config {
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	return config{
		addr:     get("LEADS_ADDR", "127.0.0.1:4001"),
		dataDir:  get("STATE_DIRECTORY", "./data"), // set by systemd StateDirectory=
		smtpHost: get("SMTP_HOST", "mail.hosting.reg.ru"),
		smtpPort: get("SMTP_PORT", "465"),
		smtpUser: os.Getenv("SMTP_USER"),
		smtpPass: os.Getenv("SMTP_PASS"),
		mailTo:   os.Getenv("MAIL_TO"),
		tgToken:  os.Getenv("TG_BOT_TOKEN"),
		tgChat:   os.Getenv("TG_CHAT_ID"),
	}
}

type record struct {
	ID         string    `json:"id"`
	ReceivedAt time.Time `json:"receivedAt"`
	IP         string    `json:"ip"` // consent evidence (152-FZ art. 9 p. 3)
	Lead       Lead      `json:"lead"`
}

type server struct {
	cfg     config
	log     *slog.Logger
	limiter *limiter
	mu      sync.Mutex // serialises appends to the leads file
	wg      sync.WaitGroup
	http    *http.Client
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := loadConfig()
	if err := os.MkdirAll(cfg.dataDir, 0o700); err != nil {
		log.Error("data dir", "err", err)
		os.Exit(1)
	}
	s := &server{cfg: cfg, log: log, limiter: newLimiter(5), http: &http.Client{Timeout: 10 * time.Second}}
	if cfg.smtpUser == "" || cfg.mailTo == "" {
		log.Warn("SMTP not configured: leads are only stored on disk")
	}
	if cfg.tgToken == "" || cfg.tgChat == "" {
		log.Warn("Telegram not configured: no notifications")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/leads", s.handleLead)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		log.Info("listening", "addr", cfg.addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen", "err", err)
			stop()
		}
	}()
	<-ctx.Done()

	shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	s.wg.Wait() // let in-flight deliveries finish
}

func (s *server) handleLead(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		problem(w, http.StatusUnsupportedMediaType, "expected application/json")
		return
	}
	ip := clientIP(r)
	if !s.limiter.allow(ip) {
		problem(w, http.StatusTooManyRequests, "too many requests")
		return
	}

	var lead Lead
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&lead); err != nil {
		problem(w, http.StatusBadRequest, "malformed body")
		return
	}
	if err := lead.normalize(); err != nil {
		problem(w, http.StatusBadRequest, err.Error()) // names the field only, no values
		return
	}

	rec := record{ID: newID(), ReceivedAt: time.Now().UTC(), IP: ip, Lead: lead}
	if err := s.store(rec); err != nil {
		s.log.Error("store lead", "id", rec.ID, "err", err)
		problem(w, http.StatusInternalServerError, "could not save the request")
		return
	}
	s.log.Info("lead stored", "id", rec.ID, "locale", lead.Locale) // no personal data in logs

	// Stored durably first; delivery runs in the background so the visitor never waits on SMTP.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.deliver(rec)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": rec.ID})
}

// store appends the record as one JSON line. shortcut: a flat file, move to Postgres when apps/api lands.
func (s *server) store(rec record) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(s.cfg.dataDir, "leads.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (s *server) deliver(rec record) {
	if s.cfg.smtpUser != "" && s.cfg.mailTo != "" {
		if err := s.sendMail(rec); err != nil {
			s.log.Error("send mail", "id", rec.ID, "err", err)
		}
	}
	if s.cfg.tgToken != "" && s.cfg.tgChat != "" {
		if err := s.notifyTelegram(rec); err != nil {
			s.log.Error("telegram", "id", rec.ID, "err", err)
		}
	}
}

func (s *server) sendMail(rec record) error {
	l := rec.Lead
	body := fmt.Sprintf("Заявка %s\nПолучена: %s UTC\n\nИмя: %s\nКлиника: %s\nТелефон: %s\nEmail: %s\nВрачей: %s\nЯзык: %s\n\nКомментарий:\n%s\n\nСогласие на обработку ПДн: да (редакция %s, политика %s), IP %s\n",
		rec.ID, rec.ReceivedAt.Format("2006-01-02 15:04"), l.Name, l.Clinic, l.Phone, dash(l.Email), dash(l.Doctors), l.Locale, dash(l.Message), l.ConsentVersion, l.PolicyVersion, rec.IP)

	headers := []string{
		"From: " + s.cfg.smtpUser,
		"To: " + s.cfg.mailTo,
		"Subject: " + mime.QEncoding.Encode("utf-8", "Заявка на демо: "+l.Clinic),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: 8bit",
		"Date: " + rec.ReceivedAt.Format(time.RFC1123Z),
	}
	if l.Email != "" { // validated: no whitespace or CRLF, so no header injection
		headers = append(headers, "Reply-To: "+l.Email)
	}
	msg := strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n")

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(s.cfg.smtpHost, s.cfg.smtpPort), &tls.Config{ServerName: s.cfg.smtpHost})
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	c, err := smtp.NewClient(conn, s.cfg.smtpHost)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()
	if err := c.Auth(smtp.PlainAuth("", s.cfg.smtpUser, s.cfg.smtpPass, s.cfg.smtpHost)); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if err := c.Mail(s.cfg.smtpUser); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, to := range strings.Split(s.cfg.mailTo, ",") {
		if err := c.Rcpt(strings.TrimSpace(to)); err != nil {
			return fmt.Errorf("rcpt: %w", err)
		}
	}
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := wc.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	return c.Quit()
}

// notifyTelegram sends a ping without personal data: Telegram stores messages outside Russia.
func (s *server) notifyTelegram(rec record) error {
	text := fmt.Sprintf("Новая заявка на демо %s\nВрачей: %s · язык: %s\nКонтакты — в почте %s", rec.ID, dash(rec.Lead.Doctors), rec.Lead.Locale, s.cfg.mailTo)
	form := url.Values{"chat_id": {s.cfg.tgChat}, "text": {text}}
	res, err := s.http.PostForm("https://api.telegram.org/bot"+s.cfg.tgToken+"/sendMessage", form)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err // url.Error embeds the URL, which contains the bot token
		}
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", res.StatusCode)
	}
	return nil
}

// clientIP trusts X-Forwarded-For only from the local reverse proxy (Caddy overwrites it with the real client).
func clientIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	return host
}

const (
	maxTrackedKeys = 10_000 // bounds memory and the pruning loop
	globalPerHour  = 60     // caps outbound mail under a distributed flood
)

type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
	global []time.Time
}

func newLimiter(max int) *limiter {
	return &limiter{max: max, window: 10 * time.Minute, hits: map[string][]time.Time{}}
}

// limitKey groups IPv6 clients by /64: one host usually controls the whole prefix.
func limitKey(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() != nil {
		return ip
	}
	return parsed.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

// allow is a sliding window per client key plus a global hourly cap.
func (l *limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	ip = limitKey(ip)

	g := l.global[:0]
	for _, t := range l.global {
		if now.Sub(t) <= time.Hour {
			g = append(g, t)
		}
	}
	l.global = g
	if len(l.global) >= globalPerHour {
		return false
	}

	for k, ts := range l.hits {
		if len(ts) == 0 || now.Sub(ts[len(ts)-1]) > l.window {
			delete(l.hits, k)
		}
	}
	if _, seen := l.hits[ip]; !seen && len(l.hits) >= maxTrackedKeys {
		return false // table full of active clients: shed new ones rather than grow without bound
	}
	recent := l.hits[ip][:0]
	for _, t := range l.hits[ip] {
		if now.Sub(t) <= l.window {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.max {
		l.hits[ip] = recent
		return false
	}
	l.hits[ip] = append(recent, now)
	l.global = append(l.global, now)
	return true
}

func problem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "title": http.StatusText(status), "detail": detail})
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
