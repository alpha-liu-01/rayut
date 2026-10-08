package traffic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLedgerAccumulatesAndResetsSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.json")
	up := true
	pid := 7
	l := New(path)
	l.alive = func() (int, bool) { return pid, up }

	first := l.Account(Sample{Up: 10, Down: 20}, 100, 200)
	if first.UploadTotal != 100 || first.DownloadTotal != 200 || first.Up != 10 || first.Down != 20 {
		t.Fatalf("session %+v", first)
	}
	if first.CumulativeUpload != 100 || first.CumulativeDownload != 200 {
		t.Fatalf("cumulative %+v", first)
	}

	next := l.Account(Sample{Up: 5, Down: 8}, 150, 260)
	if next.UploadTotal != 150 || next.DownloadTotal != 260 || next.CumulativeUpload != 150 || next.CumulativeDownload != 260 {
		t.Fatalf("delta %+v", next)
	}
	if len(next.Samples) != 2 {
		t.Fatalf("samples %d", len(next.Samples))
	}

	up = false
	closed := l.Account(Sample{Up: 5, Down: 8}, 150, 260)
	if closed.UploadTotal != 0 || closed.DownloadTotal != 0 || closed.Up != 0 || closed.Down != 0 {
		t.Fatalf("closed session %+v", closed)
	}
	if len(closed.Samples) != 0 || closed.CumulativeUpload != 150 || closed.CumulativeDownload != 260 {
		t.Fatalf("closed cumulative %+v", closed)
	}

	up = true
	pid = 8
	again := l.Account(Sample{Up: 1, Down: 1}, 10, 20)
	if again.UploadTotal != 10 || again.DownloadTotal != 20 || again.CumulativeUpload != 160 || again.CumulativeDownload != 280 {
		t.Fatalf("new session %+v", again)
	}
}

func TestLedgerReloadKeepsCumulative(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.json")
	l := New(path)
	l.alive = func() (int, bool) { return 7, true }
	l.Account(Sample{}, 40, 80)

	reloaded := New(path)
	reloaded.alive = func() (int, bool) { return 7, true }
	same := reloaded.Account(Sample{Up: 2, Down: 3}, 40, 80)
	if same.CumulativeUpload != 40 || same.CumulativeDownload != 80 || same.UploadTotal != 40 {
		t.Fatalf("reloaded %+v", same)
	}
	grown := reloaded.Account(Sample{}, 70, 90)
	if grown.CumulativeUpload != 70 || grown.CumulativeDownload != 90 || grown.UploadTotal != 70 {
		t.Fatalf("grown %+v", grown)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, secret := range []string{"super-secret-password", "example.com", "://", "uuid", "token"} {
		if strings.Contains(text, secret) {
			t.Fatalf("record contains %s: %s", secret, text)
		}
	}
}

func TestLedgerRewritesRecordWithoutSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traffic.json")
	const planted = `{"upload":40,"download":80,"sessionUpload":0,"sessionDownload":0,"password":"super-secret-password","host":"example.com"}`
	if err := os.WriteFile(path, []byte(planted), 0o600); err != nil {
		t.Fatal(err)
	}
	l := New(path)
	l.alive = func() (int, bool) { return 7, true }
	view := l.Account(Sample{}, 5, 7)
	if view.CumulativeUpload != 45 || view.CumulativeDownload != 87 {
		t.Fatalf("view %+v", view)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, "super-secret-password") || strings.Contains(text, "example.com") {
		t.Fatalf("record %s", text)
	}
}

func TestLedgerKeepsSamplesBounded(t *testing.T) {
	l := New("")
	l.alive = func() (int, bool) { return 7, true }
	for i := 0; i < sampleLimit+5; i++ {
		l.Account(Sample{Up: int64(i)}, int64(i), 0)
	}
	if got := len(l.View().Samples); got != sampleLimit {
		t.Fatalf("samples %d", got)
	}
}
