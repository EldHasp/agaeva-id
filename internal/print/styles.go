package print

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// StylesheetSource loads the FSSP site's current stylesheets.
type StylesheetSource interface {
	Links(ctx context.Context) ([]string, error)
}

// LiveStyles reads stylesheet URLs from the public FSSP page at print time.
// It does not send the person's search and does not solve a captcha.
type LiveStyles struct {
	PageURL string
	Client  *http.Client

	mu       sync.Mutex
	cached   []string
	cachedAt time.Time
}

func (s *LiveStyles) Links(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	if time.Since(s.cachedAt) < 30*time.Minute && len(s.cached) > 0 {
		out := append([]string(nil), s.cached...)
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	page := s.PageURL
	if page == "" {
		page = fsspBase
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, page, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "AgaevaId/0.1 (local print; stylesheet refresh)")
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errString("сайт ФССП не отдал страницу со стилями")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	links := extractStylesheets(page, string(body))
	if len(links) == 0 {
		return nil, errString("на странице ФССП не найдены таблицы стилей")
	}
	s.mu.Lock()
	s.cached = append([]string(nil), links...)
	s.cachedAt = time.Now()
	s.mu.Unlock()
	return links, nil
}

func extractStylesheets(page string, body string) []string {
	base, err := url.Parse(page)
	if err != nil {
		return nil
	}
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil
	}
	var links []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "link" {
			rel := strings.ToLower(attr(n, "rel"))
			href := strings.TrimSpace(attr(n, "href"))
			if strings.Contains(rel, "stylesheet") && href != "" {
				ref, err := url.Parse(href)
				if err == nil {
					links = append(links, base.ResolveReference(ref).String())
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links
}
