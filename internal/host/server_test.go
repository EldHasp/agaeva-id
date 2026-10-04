package host

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/EldHasp/cursor-global-context/apps/agaeva-id/internal/modules"
)

type stub struct{}

func (stub) Name() string  { return "print" }
func (stub) Route() string { return "/print" }
func (stub) Execute(ctx context.Context, req modules.Request) (modules.Response, error) {
	if strings.TrimSpace(req.HTML) == "" {
		return modules.Response{}, errString("в запросе нет поля html")
	}
	if strings.Contains(req.HTML, "fail") {
		return modules.Response{}, errString("печать не удалась")
	}
	return modules.Response{ContentType: "application/pdf", Body: []byte("%PDF-1.4 test")}, nil
}

type errString string

func (e errString) Error() string { return string(e) }

func TestOptionsPrivateNetwork(t *testing.T) {
	s := &Server{Version: "0.1.0", Modules: []modules.Module{stub{}}}
	req := httptest.NewRequest(http.MethodOptions, "http://127.0.0.1:17321/print", nil)
	req.RemoteAddr = "127.0.0.1:9"
	req.Header.Set("Origin", "https://portal.bitrix24.ru")
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Private-Network") != "true" {
		t.Fatal(rr.Header())
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://portal.bitrix24.ru" {
		t.Fatal(rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestPrintContract(t *testing.T) {
	s := &Server{Version: "0.1.0", Modules: []modules.Module{stub{}}}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:17321/print", strings.NewReader(`{"html":"<p>ok</p>"}`))
	req.RemoteAddr = "127.0.0.1:9"
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	if rr.Header().Get("Content-Type") != "application/pdf" {
		t.Fatal(rr.Header().Get("Content-Type"))
	}
	body, _ := io.ReadAll(rr.Body)
	if string(body) != "%PDF-1.4 test" {
		t.Fatalf("%q", body)
	}
}

func TestPrintFailureIsJSON(t *testing.T) {
	s := &Server{Version: "0.1.0", Modules: []modules.Module{stub{}}}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:17321/print", strings.NewReader(`{"html":"fail"}`))
	req.RemoteAddr = "127.0.0.1:9"
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code == 200 || strings.Contains(rr.Header().Get("Content-Type"), "pdf") {
		t.Fatalf("fake pdf: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "печать не удалась") {
		t.Fatal(rr.Body.String())
	}
}

func TestListenIsLoopbackOnly(t *testing.T) {
	if ListenAddr(17321) != "127.0.0.1:17321" {
		t.Fatal(ListenAddr(17321))
	}
	if err := ValidateAddr("0.0.0.0:17321"); err == nil {
		t.Fatal("public bind accepted")
	}
}
