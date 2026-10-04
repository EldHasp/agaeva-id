package print

import (
	"strings"
	"testing"
)

func TestPrepareFragmentUsesSiteStylesAndCropsResults(t *testing.T) {
	in := `<div class="header">шапка</div><div class="results"><table class="list"><tr><td>ИП 123</td></tr></table></div>`
	out, err := Prepare(in, []string{"https://is.fssp.gov.ru/css/app.css"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `href="https://is.fssp.gov.ru/"`) {
		t.Fatalf("base missing: %s", out)
	}
	if !strings.Contains(out, "https://is.fssp.gov.ru/css/app.css") {
		t.Fatal("stylesheet not injected")
	}
	if !strings.Contains(out, "agaeva-print-root") {
		t.Fatal("results block not marked")
	}
	if !strings.Contains(out, "A4 landscape") {
		t.Fatal("page size missing")
	}
	if !strings.Contains(out, "print-color-adjust") {
		t.Fatal("backgrounds not requested")
	}
}

func TestPrepareEmpty(t *testing.T) {
	if _, err := Prepare("  ", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractStylesheetsResolvesRelative(t *testing.T) {
	body := `<html><head><link rel="stylesheet" href="/static/main.css"><link rel="icon" href="/favicon.ico"></head></html>`
	links := extractStylesheets("https://is.fssp.gov.ru/", body)
	if len(links) != 1 || links[0] != "https://is.fssp.gov.ru/static/main.css" {
		t.Fatalf("%v", links)
	}
}
