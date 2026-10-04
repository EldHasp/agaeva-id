// Package host is the listening core. It accepts loopback requests from the
// Bitrix24 tab and hands each one to the module registered for that path.
package host

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/EldHasp/cursor-global-context/apps/agaeva-id/internal/modules"
)

const (
	maxBody     = 4 << 20
	listenHost  = "127.0.0.1"
	defaultPort = 17321
)

// Server is the user-session HTTP core.
type Server struct {
	Addr    string
	Version string
	Modules []modules.Module
	Log     *log.Logger
}

// ListenAddr is the only address the core is allowed to bind.
func ListenAddr(port int) string {
	if port == 0 {
		port = defaultPort
	}
	return net.JoinHostPort(listenHost, itoa(port))
}

// Handler is the HTTP surface. Exposed for tests.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	byRoute := map[string]modules.Module{}
	for _, m := range s.Modules {
		byRoute[m.Route()] = m
		mux.HandleFunc(m.Route(), s.dispatch(m))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopback(r) {
			writeError(w, http.StatusForbidden, "помощник принимает запросы только с этого компьютера")
			return
		}
		setCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// Run binds 127.0.0.1 and serves until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shut)
	}()
	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "нужен GET")
		return
	}
	names := make([]string, 0, len(s.Modules))
	for _, m := range s.Modules {
		names = append(names, m.Name())
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"name":    "Агаева ID",
		"version": s.Version,
		"modules": names,
	})
}

func (s *Server) dispatch(m modules.Module) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "нужен POST")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
		if err != nil {
			writeError(w, http.StatusBadRequest, "не удалось прочитать запрос")
			return
		}
		if len(body) > maxBody {
			writeError(w, http.StatusRequestEntityTooLarge, "фрагмент страницы слишком большой")
			return
		}
		var payload struct {
			HTML string `json:"html"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "тело запроса должно быть JSON")
			return
		}
		started := time.Now()
		resp, err := m.Execute(r.Context(), modules.Request{HTML: payload.HTML, Raw: body})
		if s.Log != nil {
			if err != nil {
				s.Log.Printf("module=%s status=error duration=%s message=%s", m.Name(), time.Since(started).Truncate(time.Millisecond), err.Error())
			} else {
				s.Log.Printf("module=%s status=%d bytes=%d duration=%s", m.Name(), resp.Status, len(resp.Body), time.Since(started).Truncate(time.Millisecond))
			}
		}
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		if resp.ContentType != "" {
			w.Header().Set("Content-Type", resp.ContentType)
		}
		status := resp.Status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write(resp.Body)
	}
}

func setCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Vary", "Origin")
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func loopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// Probe reports whether this address already serves Агаева ID.
func Probe(addr string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body); err != nil {
		return false
	}
	return body.Name == "Агаева ID"
}

// ErrNotLoopback is reserved so callers can recognise a refused remote bind.
var ErrNotLoopback = errors.New("listen address must be 127.0.0.1")

// ValidateAddr rejects any bind that is not the IPv4 loopback.
func ValidateAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host != listenHost && host != "localhost" {
		return ErrNotLoopback
	}
	if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
		return ErrNotLoopback
	}
	if strings.Contains(host, ":") {
		return ErrNotLoopback
	}
	return nil
}
