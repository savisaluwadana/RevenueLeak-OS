package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"revenueleak-os/internal/core"
	"revenueleak-os/internal/stripe"
)

type Server struct {
	Store         *core.Store
	Password      string
	Demo          bool
	WebDir        string
	SigningKey    []byte
	attempts      sync.Mutex
	loginAttempts map[string][]time.Time
}
type apiError struct {
	Error string `json:"error"`
}

func New(store *core.Store, password string, demo bool, webDir string) *Server {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		secret = password + "|RevenueLeakOS-signed-session-v1"
	}
	hashed := sha256.Sum256([]byte(secret))
	return &Server{Store: store, Password: password, Demo: demo, WebDir: webDir, SigningKey: hashed[:], loginAttempts: map[string][]time.Time{}}
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) { respond(w, status, apiError{msg}) }
func (s *Server) sign(in string) string {
	m := hmac.New(sha256.New, s.SigningKey)
	m.Write([]byte(in))
	return hex.EncodeToString(m.Sum(nil))
}
func (s *Server) sessionValid(r *http.Request) bool {
	if s.Demo && s.Password == "" {
		return true
	}
	cookie, err := r.Cookie("rl_session")
	if err != nil {
		return false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 3 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > expiry || expiry > time.Now().Add(25*time.Hour).Unix() {
		return false
	}
	expected := s.sign(parts[0] + "." + parts[1])
	return hmac.Equal([]byte(expected), []byte(parts[2]))
}
func (s *Server) csrf(r *http.Request) string {
	if s.Demo && s.Password == "" {
		return "demo-csrf-token"
	}
	c, err := r.Cookie("rl_session")
	if err != nil {
		return ""
	}
	return s.sign("csrf:" + c.Value)
}
func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/me" && r.URL.Path != "/api/login" && !s.sessionValid(r) {
			fail(w, 401, "Please sign in to continue")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != "GET" && r.Method != "HEAD" && r.URL.Path != "/api/login" {
			if !s.sessionValid(r) {
				fail(w, 401, "Unauthorized")
				return
			}
			if !hmac.Equal([]byte(r.Header.Get("X-CSRF-Token")), []byte(s.csrf(r))) {
				fail(w, 403, "Invalid CSRF token")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/me", s.me)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("GET /api/overview", s.overview)
	mux.HandleFunc("GET /api/findings", s.findings)
	mux.HandleFunc("GET /api/findings/export", s.export)
	mux.HandleFunc("POST /api/findings/{id}/status", s.updateFinding)
	mux.HandleFunc("GET /api/datasets", s.datasets)
	mux.HandleFunc("GET /api/audit", s.audit)
	mux.HandleFunc("POST /api/import/{kind}", s.importCSV)
	mux.HandleFunc("POST /api/scan", s.scan)
	mux.HandleFunc("POST /api/stripe/sync", s.stripeSync)
	mux.HandleFunc("POST /api/demo/reset", s.resetDemo)
	fs := http.FileServer(http.Dir(s.WebDir))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, s.WebDir+"/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	})
	return s.security(mux)
}
func parseJSON(r *http.Request, v any) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return errors.New("Content-Type must be application/json")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 8192))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var rest any
	if err := dec.Decode(&rest); err != io.EOF {
		return errors.New("only one JSON value is allowed")
	}
	return nil
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	authenticated := s.sessionValid(r)
	respond(w, 200, map[string]any{"authenticated": authenticated, "demo": s.Demo, "csrf": func() string {
		if authenticated {
			return s.csrf(r)
		}
		return ""
	}(), "stripe_configured": os.Getenv("STRIPE_RESTRICTED_KEY") != "" && os.Getenv("STRIPE_PRICE_MAP") != "", "version": "0.1.0"})
}
func (s *Server) limitLogin(r *http.Request) bool {
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i >= 0 {
		ip = ip[:i]
	}
	now := time.Now()
	s.attempts.Lock()
	defer s.attempts.Unlock()
	recent := []time.Time{}
	for _, x := range s.loginAttempts[ip] {
		if now.Sub(x) < time.Minute {
			recent = append(recent, x)
		}
	}
	if len(recent) >= 10 {
		s.loginAttempts[ip] = recent
		return false
	}
	s.loginAttempts[ip] = append(recent, now)
	return true
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.limitLogin(r) {
		fail(w, 429, "Too many login attempts; retry later")
		return
	}
	if s.Password == "" {
		fail(w, 400, "No admin password configured")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := parseJSON(r, &req); err != nil {
		fail(w, 400, "Invalid JSON payload")
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.Password)) != 1 {
		fail(w, 401, "Invalid credentials")
		return
	}
	nonce := make([]byte, 20)
	if _, err := rand.Read(nonce); err != nil {
		fail(w, 500, "Secure randomness unavailable")
		return
	}
	raw := fmt.Sprintf("%d.%x", time.Now().Add(24*time.Hour).Unix(), nonce)
	cookie := &http.Cookie{Name: "rl_session", Value: raw + "." + s.sign(raw), Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: 86400}
	http.SetCookie(w, cookie)
	respond(w, 200, map[string]any{"ok": true, "csrf": s.sign("csrf:" + cookie.Value)})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "rl_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	d := s.Store.Snapshot()
	type totals struct {
		PotentialCents int64 `json:"potential_cents"`
		Open           int   `json:"open"`
		Reviewing      int   `json:"reviewing"`
		Approved       int   `json:"approved"`
		Resolved       int   `json:"resolved"`
		FalsePositive  int   `json:"false_positive"`
	}
	byCurrency := map[string]*totals{}
	byKind := map[string]int{}
	for _, f := range d.Findings {
		t := byCurrency[f.Currency]
		if t == nil {
			t = &totals{}
			byCurrency[f.Currency] = t
		}
		switch f.Status {
		case "new":
			t.Open++
			t.PotentialCents += f.PotentialCents
		case "reviewing":
			t.Reviewing++
			t.PotentialCents += f.PotentialCents
		case "approved":
			t.Approved++
			t.PotentialCents += f.PotentialCents
		case "resolved":
			t.Resolved++
		case "false_positive":
			t.FalsePositive++
		}
		byKind[f.Kind]++
	}
	respond(w, 200, map[string]any{"by_currency": byCurrency, "by_kind": byKind, "findings_count": len(d.Findings), "customers": countCustomers(d.Contracts), "last_scan": d.LastScan, "data_counts": map[string]int{"contracts": len(d.Contracts), "usage": len(d.Usage), "billing": len(d.Billing), "credits": len(d.Credits)}, "updated_at": d.UpdatedAt})
}
func countCustomers(rows []core.Contract) int {
	m := map[string]bool{}
	for _, v := range rows {
		m[v.CustomerID] = true
	}
	return len(m)
}
func (s *Server) findings(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, map[string]any{"findings": s.Store.Snapshot().Findings})
}
func (s *Server) datasets(w http.ResponseWriter, r *http.Request) {
	d := s.Store.Snapshot()
	respond(w, 200, map[string]any{"contracts": d.Contracts, "usage": d.Usage, "billing": d.Billing, "credits": d.Credits})
}
func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, map[string]any{"events": s.Store.Snapshot().Audit})
}
func (s *Server) scan(w http.ResponseWriter, r *http.Request) {
	var summary core.ScanSummary
	err := s.Store.Update(func(d *core.Database) error {
		summary = core.Scan(d, time.Now(), 7)
		core.Log(d, "scan.complete", fmt.Sprintf("%d findings identified", summary.Findings))
		return nil
	})
	if err != nil {
		log.Printf("scan persistence: %v", err)
		fail(w, 500, "Could not save scan")
		return
	}
	respond(w, 200, summary)
}
func (s *Server) importCSV(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if kind != "contracts" && kind != "usage" && kind != "billing" && kind != "credits" {
		fail(w, 404, "Unknown dataset type")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		fail(w, 400, "Upload must be a CSV file smaller than 2MB")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		fail(w, 400, "Missing CSV file")
		return
	}
	defer file.Close()
	parsed, err := core.ReadCSV(kind, file)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	var result core.ImportResult
	err = s.Store.Update(func(d *core.Database) error {
		result = core.Import(d, kind, parsed)
		core.Scan(d, time.Now(), 7)
		core.Log(d, "import."+kind, fmt.Sprintf("%d added, %d updated", result.Added, result.Updated))
		return nil
	})
	if err != nil {
		log.Printf("import persistence: %v", err)
		fail(w, 500, "Could not save dataset")
		return
	}
	respond(w, 200, result)
}
func (s *Server) updateFinding(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := parseJSON(r, &req); err != nil {
		fail(w, 400, "Invalid JSON payload")
		return
	}
	allowed := map[string]bool{"new": true, "reviewing": true, "approved": true, "false_positive": true, "resolved": true}
	if !allowed[req.Status] || len(req.Note) > 1500 {
		fail(w, 400, "Invalid status or note too long")
		return
	}
	id := r.PathValue("id")
	var result core.Finding
	err := s.Store.Update(func(d *core.Database) error {
		for i := range d.Findings {
			if d.Findings[i].ID == id {
				d.Findings[i].Status = req.Status
				d.Findings[i].Note = req.Note
				result = d.Findings[i]
				core.Log(d, "finding.status", id+" -> "+req.Status)
				return nil
			}
		}
		return errors.New("not found")
	})
	if err != nil {
		if err.Error() == "not found" {
			fail(w, 404, "Finding not found")
			return
		}
		log.Printf("status persistence: %v", err)
		fail(w, 500, "Unable to update finding")
		return
	}
	respond(w, 200, result)
}
func (s *Server) stripeSync(w http.ResponseWriter, r *http.Request) {
	client, err := stripe.NewFromEnv()
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	ctx := r.Context()
	result, err := client.Fetch(ctx)
	if err != nil {
		log.Printf("stripe sync: %v", err)
		fail(w, 502, "Stripe sync failed; review server logs and restricted key permissions")
		return
	}
	// Stripe is the authority for previously synchronized records: remove records no longer present in the fetched snapshot.
	// API pagination safety: Fetch must complete before this write is ever attempted.
	err = s.Store.Update(func(d *core.Database) error {
		kept := make([]core.Billing, 0, len(d.Billing))
		for _, b := range d.Billing {
			if b.Source != "stripe" {
				kept = append(kept, b)
			}
		}
		d.Billing = append(kept, result.Records...)
		core.Scan(d, time.Now(), 7)
		core.Log(d, "stripe.sync", fmt.Sprintf("%d invoices, %d mapped lines, %d skipped unmapped", result.Invoices, result.Lines, result.SkippedUnmapped))
		return nil
	})
	if err != nil {
		log.Printf("stripe persistence: %v", err)
		fail(w, 500, "Could not save Stripe sync")
		return
	}
	respond(w, 200, result)
}
func (s *Server) resetDemo(w http.ResponseWriter, r *http.Request) {
	if !s.Demo {
		fail(w, 403, "Demo reset unavailable outside demo mode")
		return
	}
	err := s.Store.Update(func(d *core.Database) error { *d = core.NewDatabase(); core.SeedDemo(d, time.Now()); return nil })
	if err != nil {
		fail(w, 500, "Could not reset demo")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func safeCSV(s string) string {
	if strings.HasPrefix(s, "=") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "@") || strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") {
		return "'" + s
	}
	return s
}
func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=revenueleak-findings.csv")
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"id", "customer_id", "customer_name", "period", "metric", "kind", "currency", "potential_cents", "expected_cents", "billed_cents", "status", "note"})
	for _, f := range s.Store.Snapshot().Findings {
		_ = writer.Write([]string{safeCSV(f.ID), safeCSV(f.CustomerID), safeCSV(f.CustomerName), f.Period, safeCSV(f.Metric), f.Kind, f.Currency, strconv.FormatInt(f.PotentialCents, 10), strconv.FormatInt(f.ExpectedCents, 10), strconv.FormatInt(f.BilledCents, 10), f.Status, safeCSV(f.Note)})
	}
	writer.Flush()
}
