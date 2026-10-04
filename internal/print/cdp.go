package print

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// PrintHTML renders document with the installed browser and returns PDF bytes.
func PrintHTML(ctx context.Context, browserPath string, document string) ([]byte, error) {
	profile, err := os.MkdirTemp("", "agaeva-id-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(profile)

	cmd := exec.CommandContext(ctx, browserPath,
		"--headless=new",
		"--disable-gpu",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-sync",
		"--hide-scrollbars",
		"--window-size=1400,900",
		"--user-data-dir="+profile,
		"--remote-debugging-port=0",
		"about:blank",
	)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("не удалось запустить браузер: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	devURL, err := waitDevTools(ctx, stderr)
	if err != nil {
		return nil, err
	}
	pageWS, err := newPageSocket(ctx, devURL)
	if err != nil {
		return nil, err
	}
	conn, err := dialWebSocket(ctx, pageWS)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := &cdp{conn: conn}
	if _, err := client.call(ctx, "Page.enable", map[string]any{}); err != nil {
		return nil, err
	}
	tree, err := client.call(ctx, "Page.getFrameTree", map[string]any{})
	if err != nil {
		return nil, err
	}
	frameID := digFrameID(tree)
	if frameID == "" {
		return nil, errString("браузер не открыл страницу для печати")
	}
	if _, err := client.call(ctx, "Page.setDocumentContent", map[string]any{
		"frameId": frameID,
		"html":    document,
	}); err != nil {
		return nil, err
	}
	_, _ = client.call(ctx, "Emulation.setEmulatedMedia", map[string]any{"media": "screen"})
	_, _ = client.call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{
		"width": 1400, "height": 900, "deviceScaleFactor": 1, "mobile": false,
	})
	_ = client.waitStyles(ctx)

	raw, err := client.call(ctx, "Page.printToPDF", map[string]any{
		"landscape":           true,
		"printBackground":     true,
		"paperWidth":          11.6929,
		"paperHeight":         8.2677,
		"marginTop":           0.32,
		"marginBottom":        0.32,
		"marginLeft":          0.32,
		"marginRight":         0.32,
		"preferCSSPageSize":   true,
		"displayHeaderFooter": false,
		"scale":               1,
	})
	if err != nil {
		return nil, fmt.Errorf("печать не удалась: %w", err)
	}
	var parsed struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed.Data == "" {
		return nil, errString("браузер не вернул PDF")
	}
	pdf, err := base64.StdEncoding.DecodeString(parsed.Data)
	if err != nil || len(pdf) < 5 || string(pdf[:5]) != "%PDF-" {
		return nil, errString("браузер вернул не PDF")
	}
	return pdf, nil
}

func waitDevTools(ctx context.Context, stderr io.Reader) (string, error) {
	lines := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timer.C:
			return "", errString("браузер не открыл порт печати")
		case line, ok := <-lines:
			if !ok {
				return "", errString("браузер закрылся до начала печати")
			}
			const mark = "DevTools listening on "
			if i := strings.Index(line, mark); i >= 0 {
				return strings.TrimSpace(line[i+len(mark):]), nil
			}
		}
	}
}

func newPageSocket(ctx context.Context, browserWS string) (string, error) {
	// ws://127.0.0.1:port/devtools/browser/...
	u := strings.TrimPrefix(browserWS, "ws://")
	host, _, ok := strings.Cut(u, "/")
	if !ok {
		return "", errString("непонятный адрес браузера")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://"+host+"/json/new?about:blank", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Older Chrome accepts GET.
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+"/json/new?about:blank", nil)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var page struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &page); err != nil || page.WebSocketDebuggerURL == "" {
		return "", errString("браузер не создал вкладку для печати")
	}
	return page.WebSocketDebuggerURL, nil
}

type cdp struct {
	conn *wsConn
	next int
}

func (c *cdp) call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	c.next++
	id := c.next
	msg, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	if err := c.conn.sendText(msg); err != nil {
		return nil, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(45 * time.Second)
	}
	for {
		if time.Now().After(deadline) {
			return nil, errString("браузер не ответил вовремя")
		}
		payload, err := c.conn.readText(deadline)
		if err != nil {
			return nil, err
		}
		var env struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(payload, &env); err != nil || env.ID == 0 {
			continue
		}
		if env.ID != id {
			continue
		}
		if env.Error != nil {
			return nil, errString(env.Error.Message)
		}
		return env.Result, nil
	}
}

func (c *cdp) waitStyles(ctx context.Context) error {
	deadline := time.Now().Add(8 * time.Second)
	expr := `(() => {
		const links = [...document.querySelectorAll('link[rel="stylesheet"]')];
		if (links.length === 0) return true;
		return links.every((l) => l.sheet);
	})()`
	for time.Now().Before(deadline) {
		raw, err := c.call(ctx, "Runtime.evaluate", map[string]any{
			"expression":    expr,
			"returnByValue": true,
		})
		if err == nil {
			var parsed struct {
				Result struct {
					Value bool `json:"value"`
				} `json:"result"`
			}
			if json.Unmarshal(raw, &parsed) == nil && parsed.Result.Value {
				return nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return nil
}

func digFrameID(raw json.RawMessage) string {
	var tree struct {
		FrameTree struct {
			Frame struct {
				ID string `json:"id"`
			} `json:"frame"`
		} `json:"frameTree"`
	}
	if json.Unmarshal(raw, &tree) == nil {
		return tree.FrameTree.Frame.ID
	}
	return ""
}

type wsConn struct {
	c net.Conn
	r *bufio.Reader
}

func dialWebSocket(ctx context.Context, wsURL string) (*wsConn, error) {
	rest := strings.TrimPrefix(wsURL, "ws://")
	host, path, ok := strings.Cut(rest, "/")
	if !ok {
		return nil, errString("непонятный адрес вкладки")
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	req := fmt.Sprintf("GET /%s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		path, host, base64.StdEncoding.EncodeToString(key))
	if _, err := io.WriteString(conn, req); err != nil {
		conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !strings.Contains(status, "101") {
		conn.Close()
		return nil, errString("браузер отказал в подключении к печати")
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}
	return &wsConn{c: conn, r: br}, nil
}

func (w *wsConn) Close() error { return w.c.Close() }

func (w *wsConn) sendText(payload []byte) error {
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	header := []byte{0x81}
	n := len(payload)
	switch {
	case n < 126:
		header = append(header, byte(0x80|n))
	case n < 65536:
		header = append(header, 0x80|126, byte(n>>8), byte(n))
	default:
		header = append(header, 0x80|127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(n))
		header = append(header, ext[:]...)
	}
	header = append(header, mask...)
	masked := make([]byte, n)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	if _, err := w.c.Write(header); err != nil {
		return err
	}
	_, err := w.c.Write(masked)
	return err
}

func (w *wsConn) readText(deadline time.Time) ([]byte, error) {
	_ = w.c.SetReadDeadline(deadline)
	var assembled []byte
	for {
		b0, err := w.r.ReadByte()
		if err != nil {
			return nil, err
		}
		b1, err := w.r.ReadByte()
		if err != nil {
			return nil, err
		}
		opcode := b0 & 0x0f
		masked := b1&0x80 != 0
		length := int(b1 & 0x7f)
		switch length {
		case 126:
			var ext [2]byte
			if _, err := io.ReadFull(w.r, ext[:]); err != nil {
				return nil, err
			}
			length = int(binary.BigEndian.Uint16(ext[:]))
		case 127:
			var ext [8]byte
			if _, err := io.ReadFull(w.r, ext[:]); err != nil {
				return nil, err
			}
			length = int(binary.BigEndian.Uint64(ext[:]))
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(w.r, mask[:]); err != nil {
				return nil, err
			}
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(w.r, payload); err != nil {
			return nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		switch opcode {
		case 0x8:
			return nil, io.EOF
		case 0x9:
			_ = w.sendPong(payload)
			continue
		case 0xA:
			continue
		case 0x1, 0x0:
			assembled = append(assembled, payload...)
			if b0&0x80 != 0 {
				return assembled, nil
			}
		default:
			return nil, errors.New("неожиданный кадр браузера")
		}
	}
}

func (w *wsConn) sendPong(payload []byte) error {
	frame := []byte{0x8A, byte(len(payload))}
	frame = append(frame, payload...)
	_, err := w.c.Write(frame)
	return err
}
