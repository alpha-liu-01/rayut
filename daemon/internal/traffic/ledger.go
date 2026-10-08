package traffic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/alpha-liu-01/rayut/daemon/internal/core"
	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
)

const sampleLimit = 30

// Sample is one MetaCubeXD traffic point: up and down are bytes per second.
type Sample struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// View is the home-page snapshot. uploadTotal and downloadTotal are this
// core session. up and down are the latest rates. Cumulative totals survive
// a new session.
type View struct {
	UploadTotal        int64    `json:"uploadTotal"`
	DownloadTotal      int64    `json:"downloadTotal"`
	Up                 int64    `json:"up"`
	Down               int64    `json:"down"`
	CumulativeUpload   int64    `json:"cumulativeUpload"`
	CumulativeDownload int64    `json:"cumulativeDownload"`
	Samples            []Sample `json:"samples"`
}

// record is the only on-disk shape. It stores byte counters, not destinations.
type record struct {
	Upload          int64 `json:"upload"`
	Download        int64 `json:"download"`
	SessionUpload   int64 `json:"sessionUpload"`
	SessionDownload int64 `json:"sessionDownload"`
	Pid             int   `json:"pid"`
}

// Ledger folds mihomo session totals into a cumulative file.
type Ledger struct {
	mu         sync.Mutex
	path       string
	alive      func() (int, bool)
	cumulative record
	open       bool
	rateUp     int64
	rateDown   int64
	samples    []Sample
}

func Path() string {
	if paths.Profile == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Dir(paths.Profile)), "traffic.json")
}

func New(path string) *Ledger {
	l := &Ledger{path: path, alive: core.Alive}
	l.load()
	if pid, ok := l.alive(); ok && pid != 0 && pid == l.cumulative.Pid {
		l.open = true
	}
	return l
}

// Account applies one sample when the core is up, and closes the session
// when it is not. A failed read must not call this with zero totals while
// the core is still up, or the next sample would be counted twice.
func (l *Ledger) Account(sample Sample, uploadTotal, downloadTotal int64) View {
	l.mu.Lock()
	defer l.mu.Unlock()
	pid, ok := l.alive()
	if !ok {
		l.end()
		return l.view()
	}
	l.apply(pid, uploadTotal, downloadTotal, sample)
	return l.view()
}

// View returns the current numbers without changing them.
func (l *Ledger) View() View {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.view()
}

func (l *Ledger) apply(pid int, uploadTotal, downloadTotal int64, sample Sample) {
	before := l.cumulative
	if l.cumulative.Pid != pid {
		l.cumulative.Upload += uploadTotal
		l.cumulative.Download += downloadTotal
		l.cumulative.Pid = pid
	} else {
		deltaUp := uploadTotal - l.cumulative.SessionUpload
		deltaDown := downloadTotal - l.cumulative.SessionDownload
		if deltaUp < 0 || deltaDown < 0 {
			deltaUp = uploadTotal
			deltaDown = downloadTotal
		}
		l.cumulative.Upload += deltaUp
		l.cumulative.Download += deltaDown
	}
	l.cumulative.SessionUpload = uploadTotal
	l.cumulative.SessionDownload = downloadTotal
	l.rateUp = sample.Up
	l.rateDown = sample.Down
	l.open = true
	l.samples = append(l.samples, sample)
	if len(l.samples) > sampleLimit {
		l.samples = append([]Sample(nil), l.samples[len(l.samples)-sampleLimit:]...)
	}
	if l.cumulative != before {
		l.persist()
	}
}

func (l *Ledger) end() {
	if !l.open {
		return
	}
	l.open = false
	l.cumulative.SessionUpload = 0
	l.cumulative.SessionDownload = 0
	l.cumulative.Pid = 0
	l.rateUp = 0
	l.rateDown = 0
	l.samples = nil
	l.persist()
}

func (l *Ledger) view() View {
	samples := append([]Sample{}, l.samples...)
	out := View{
		CumulativeUpload:   l.cumulative.Upload,
		CumulativeDownload: l.cumulative.Download,
		Samples:            samples,
	}
	if l.open {
		out.UploadTotal = l.cumulative.SessionUpload
		out.DownloadTotal = l.cumulative.SessionDownload
		out.Up = l.rateUp
		out.Down = l.rateDown
	}
	return out
}

func (l *Ledger) load() {
	if l.path == "" {
		return
	}
	body, err := os.ReadFile(l.path)
	if err != nil {
		return
	}
	var saved record
	if json.Unmarshal(body, &saved) != nil {
		return
	}
	if saved.Upload < 0 || saved.Download < 0 || saved.SessionUpload < 0 || saved.SessionDownload < 0 || saved.Pid < 0 {
		return
	}
	l.cumulative = saved
	l.persist()
}

func (l *Ledger) persist() {
	if l.path == "" {
		return
	}
	body, err := json.Marshal(l.cumulative)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return
	}
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, l.path)
}
