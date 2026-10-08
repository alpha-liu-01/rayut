package core

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/alpha-liu-01/rayut/daemon/internal/redact"
)

const sessionLogLines = 200

// LogLine is one redacted mihomo log record. The fields match the log stream
// MetaCubeXD reads: type and payload.
type LogLine struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

func openSessionLog() (io.WriteCloser, error) {
	file, err := os.OpenFile(logFile(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &lineRedactor{dst: file}, nil
}

// SessionLog returns the tail of this session's on-disk log.
// Missing the file is an empty list, not an error.
func SessionLog() ([]LogLine, error) {
	data, err := os.ReadFile(logFile())
	if err != nil {
		if os.IsNotExist(err) {
			return []LogLine{}, nil
		}
		return nil, err
	}
	if len(data) > 256*1024 {
		data = data[len(data)-256*1024:]
		if cut := bytes.IndexByte(data, '\n'); cut >= 0 {
			data = data[cut+1:]
		}
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	raw := strings.Split(text, "\n")
	lines := make([]LogLine, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(stripANSI(line))
		if line == "" {
			continue
		}
		kind, payload := parseLogLine(redact.Text(line))
		lines = append(lines, LogLine{Type: kind, Payload: payload})
	}
	if len(lines) > sessionLogLines {
		lines = lines[len(lines)-sessionLogLines:]
	}
	return lines, nil
}

type lineRedactor struct {
	dst *os.File
	buf []byte
}

func (w *lineRedactor) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		cut := bytes.IndexByte(w.buf, '\n')
		if cut < 0 {
			break
		}
		line := redact.Text(string(w.buf[:cut]))
		if _, err := w.dst.Write([]byte(line + "\n")); err != nil {
			return 0, err
		}
		w.buf = append([]byte(nil), w.buf[cut+1:]...)
	}
	return len(p), nil
}

func (w *lineRedactor) Close() error {
	if len(w.buf) > 0 {
		line := redact.Text(string(w.buf))
		if _, err := w.dst.Write([]byte(line + "\n")); err != nil {
			_ = w.dst.Close()
			return err
		}
		w.buf = nil
	}
	return w.dst.Close()
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(line string) string {
	return ansiPattern.ReplaceAllString(line, "")
}

func parseLogLine(line string) (string, string) {
	kind := "info"
	if i := strings.Index(line, "level="); i >= 0 {
		rest := line[i+len("level="):]
		end := strings.IndexAny(rest, " \t")
		if end < 0 {
			kind = rest
		} else if end > 0 {
			kind = rest[:end]
		}
	}
	const key = "msg="
	if i := strings.Index(line, key); i >= 0 {
		payload := strings.TrimSpace(line[i+len(key):])
		payload = strings.Trim(payload, `"`)
		return kind, payload
	}
	return kind, line
}
