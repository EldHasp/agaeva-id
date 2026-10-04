// Package update watches GitHub releases and replaces the installed program.
// It never logs download URLs that could carry a token.
package update

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const assetName = "AgaevaId-win-x64.zip"

// ReleasesRepository is the public repository that holds only version archives.
// Computers that receive the program read it without a token. Source stays in the private repository.
const ReleasesRepository = "EldHasp/agaeva-id-releases"

// Config is the public release the installed copy follows.
type Config struct {
	Repository    string
	Asset         string
	Interval      time.Duration
	Current       string
	Client        *http.Client
	InstallDir    string
	Executable    string
	Logf          func(string, ...any)
}

// Watch checks for a newer release until ctx ends. The first look is delayed
// so a just-started session is not interrupted.
func Watch(ctx context.Context, cfg Config) {
	if cfg.Interval <= 0 {
		cfg.Interval = 6 * time.Hour
	}
	if cfg.Asset == "" {
		cfg.Asset = assetName
	}
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := step(ctx, cfg); err != nil {
				cfg.Logf("обновление пропущено: %s", err.Error())
			}
			timer.Reset(cfg.Interval)
		}
	}
}

func step(ctx context.Context, cfg Config) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	rel, err := latest(ctx, cfg)
	if err != nil {
		return err
	}
	if !newer(rel.Tag, cfg.Current) {
		return nil
	}
	var assetURL string
	for _, a := range rel.Assets {
		if a.Name == cfg.Asset {
			assetURL = a.URL
			break
		}
	}
	if assetURL == "" {
		return errors.New("в релизе нет архива " + cfg.Asset)
	}
	dir, err := os.MkdirTemp("", "agaeva-update-")
	if err != nil {
		return err
	}
	zipPath := filepath.Join(dir, cfg.Asset)
	if err := download(ctx, cfg, assetURL, zipPath); err != nil {
		return err
	}
	staging := filepath.Join(cfg.InstallDir, "staging")
	_ = os.RemoveAll(staging)
	if err := unzip(zipPath, staging); err != nil {
		return err
	}
	next := filepath.Join(staging, "agaeva-id.exe")
	if _, err := os.Stat(next); err != nil {
		return errors.New("в архиве нет agaeva-id.exe")
	}
	script := filepath.Join(dir, "apply-update.cmd")
	body := fmt.Sprintf("@echo off\r\ntimeout /t 2 /nobreak >nul\r\nsc stop AgaevaId\r\ntimeout /t 3 /nobreak >nul\r\ncopy /Y \"%s\" \"%s\"\r\nsc start AgaevaId\r\n",
		next, cfg.Executable)
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		return err
	}
	cfg.Logf("найдена новая версия, перезапуск для обновления")
	return applyScript(script)
}

type release struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func latest(ctx context.Context, cfg Config) (release, error) {
	var rel release
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	endpoint := "https://api.github.com/repos/" + cfg.Repository + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return rel, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "AgaevaId")
	resp, err := client.Do(req)
	if err != nil {
		return rel, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return rel, errors.New("релиз недоступен")
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return rel, err
	}
	return rel, nil
}

func download(ctx context.Context, cfg Config, assetURL, dest string) error {
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "AgaevaId")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("архив обновления недоступен")
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, io.LimitReader(resp.Body, 200<<20))
	return err
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if strings.Contains(name, "..") {
			return errors.New("архив обновления отклонён")
		}
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) && target != filepath.Clean(dest) {
			return errors.New("архив обновления отклонён")
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			in.Close()
			return err
		}
		_, err = io.Copy(out, io.LimitReader(in, 200<<20))
		out.Close()
		in.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func newer(tag, current string) bool {
	return compareSemver(normalize(tag), normalize(current)) > 0
}

func normalize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "agaeva-id-")
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

func compareSemver(a, b string) int {
	ap := splitVer(a)
	bp := splitVer(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return 0
}

func splitVer(v string) [3]int {
	var out [3]int
	parts := strings.SplitN(v, ".", 3)
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}
