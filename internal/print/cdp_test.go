package print

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPrintHTMLSmoke(t *testing.T) {
	path, name, err := FindBrowser()
	if err != nil {
		t.Skip(err)
	}
	t.Log(name, path)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	html := `<div class="results"><table class="list"><tr><td style="background:#cde">ИП</td></tr></table></div>`
	doc, err := Prepare(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := PrintHTML(ctx, path, doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(pdf[:5]), "%PDF-") {
		t.Fatalf("not pdf, %d bytes", len(pdf))
	}
	if len(pdf) < 500 {
		t.Fatalf("pdf too small: %d", len(pdf))
	}
	_ = os.WriteFile("/tmp/agaeva-smoke.pdf", pdf, 0o644)
}
