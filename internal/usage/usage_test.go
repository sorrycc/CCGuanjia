package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func reply(at time.Time, id, model, cwd string, in, out int) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"requestId":"r%s","sessionId":"s-%s","cwd":%q,"message":{"id":%q,"model":%q,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_creation_input_tokens":1,"cache_read_input_tokens":2}}}`,
		at.UTC().Format(time.RFC3339Nano), id, cwd, cwd, id, model, in, out)
}

func TestScan(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	p := filepath.Join(dir, "projects", "-a", "s.jsonl")
	os.MkdirAll(filepath.Dir(p), 0o755)
	lines := []string{
		`{"type":"user","message":{"content":"hi"}}`,
		reply(now.Add(-1*time.Hour), "m1", "claude-opus", "/a", 10, 5),
		// The same reply, as streaming writes it again.
		reply(now.Add(-1*time.Hour), "m1", "claude-opus", "/a", 10, 5),
		reply(now.Add(-20*time.Hour), "m2", "claude-haiku", "/a", 100, 50),
		reply(now.Add(-48*time.Hour), "m3", "claude-opus", "/a", 1000, 0),
		`{"type":"assistant","timestamp":"2026-10-19T11:00:00Z","message":{"id":"x","model":"<synthetic>","usage":{"input_tokens":9}}}`,
		`not json "usage" "assistant"`,
	}
	os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	q := filepath.Join(dir, "projects", "-b", "t.jsonl")
	os.MkdirAll(filepath.Dir(q), 0o755)
	os.WriteFile(q, []byte(reply(now.Add(-2*time.Hour), "m4", "claude-opus", "/b", 7, 3)+"\n"), 0o600)

	s := NewScanner()
	sum := s.Scan(dir, now.Add(-24*time.Hour), now, 24)
	if sum.Replies != 3 || sum.Tokens.Input != 117 || sum.Tokens.Output != 58 || sum.Tokens.Total() != 117+58+9 {
		t.Errorf("summary %+v", sum)
	}
	if sum.Sessions != 2 {
		t.Errorf("sessions %d", sum.Sessions)
	}
	if sum.Models[0].Name != "claude-haiku" || sum.Models[0].Tokens != 153 || len(sum.Models) != 2 {
		t.Errorf("models %+v", sum.Models)
	}
	if sum.Projects[0].Name != "/a" || len(sum.Projects) != 2 {
		t.Errorf("projects %+v", sum.Projects)
	}
	if sum.Buckets[23] != 18 || sum.Buckets[22] != 13 || sum.Buckets[4] != 153 {
		t.Errorf("buckets %v", sum.Buckets)
	}
	// A wider window, from the cache.
	if got := s.Scan(dir, now.Add(-72*time.Hour), now, 3); got.Tokens.Input != 1117 {
		t.Errorf("72h %+v", got.Tokens)
	}
	// A file that changes is read again.
	f, _ := os.OpenFile(q, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(reply(now.Add(-time.Minute), "m5", "claude-opus", "/b", 1, 1) + "\n")
	f.Close()
	os.Chtimes(q, now, now)
	if got := s.Scan(dir, now.Add(-24*time.Hour), now, 24); got.Replies != 4 {
		t.Errorf("after append: %d replies", got.Replies)
	}
}

func TestStorage(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "projects", "-Users-a-x"), 0o755)
	os.WriteFile(filepath.Join(dir, "projects", "-Users-a-x", "s.jsonl"), []byte(`{"type":"user","cwd":"/Users/a/x-y"}`+"\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "history.jsonl"), []byte("12345"), 0o600)
	os.Symlink(t.TempDir(), filepath.Join(dir, "skills"))
	st := MeasureStorage(dir)
	if len(st.ByType) != 2 || st.ByType[0].Name != "projects" || st.Total != st.ByType[0].Bytes+5 {
		t.Errorf("by type %+v", st)
	}
	if len(st.ByProject) != 1 || st.ByProject[0].Path != "/Users/a/x-y" {
		t.Errorf("by project %+v", st.ByProject)
	}
}
