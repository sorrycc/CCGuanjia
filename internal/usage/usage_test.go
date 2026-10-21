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
	// The sessions, the latest first, with the model of most tokens.
	if len(sum.Recent) != 2 || sum.Recent[0].ID != "s-/a" || sum.Recent[0].Replies != 2 || sum.Recent[0].Model != "claude-haiku" ||
		!sum.Recent[0].Last.Equal(now.Add(-time.Hour)) || sum.Recent[1].Project != "/b" || sum.Recent[1].Tokens != 13 {
		t.Errorf("recent %+v", sum.Recent)
	}
	hours := int64(0)
	for _, day := range sum.Hours {
		for _, n := range day {
			hours += n
		}
	}
	if at := now.Add(-time.Hour).Local(); hours != 3 || sum.Hours[at.Weekday()][at.Hour()] != 1 {
		t.Errorf("hours %v", sum.Hours)
	}
	// The day before, which starts with m3, 48 hours ago.
	if sum.Prev.Replies != 1 || sum.Prev.Tokens.Input != 1000 || sum.Prev.Sessions != 1 {
		t.Errorf("prev %+v", sum.Prev)
	}
	if got := s.Scan(dir, now.Add(-12*time.Hour), now, 12); got.Prev.Replies != 1 || got.Prev.Tokens.Input != 100 {
		t.Errorf("prev of 12h %+v", got.Prev)
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

func TestLastActive(t *testing.T) {
	dir := t.TempDir()
	if !LastActive(dir).IsZero() {
		t.Error("an empty folder was active")
	}
	at := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	for i, name := range []string{"-a/s.jsonl", "-b/t.jsonl", "-b/notes.txt"} {
		p := filepath.Join(dir, "projects", name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, nil, 0o600)
		os.Chtimes(p, at.Add(time.Duration(i)*time.Hour), at.Add(time.Duration(i)*time.Hour))
	}
	if got := LastActive(dir); !got.Equal(at.Add(time.Hour)) {
		t.Errorf("last active %v", got)
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

func TestMerge(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	a, b := t.TempDir(), t.TempDir()
	for dir, lines := range map[string][]string{
		a: {reply(now.Add(-time.Hour), "m1", "claude-opus", "/x", 10, 5), reply(now.Add(-30*time.Hour), "m2", "claude-opus", "/x", 1, 1)},
		b: {reply(now.Add(-2*time.Hour), "m3", "claude-haiku", "/x", 100, 50), reply(now.Add(-3*time.Hour), "m4", "claude-haiku", "/y", 1, 1)},
	} {
		p := filepath.Join(dir, "projects", "-x", "s.jsonl")
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	}
	s := NewScanner()
	sum := Merge(map[string]Summary{
		"a": s.Scan(a, now.Add(-24*time.Hour), now, 24),
		"b": s.Scan(b, now.Add(-24*time.Hour), now, 24),
	})
	if sum.Replies != 3 || sum.Sessions != 3 || sum.Tokens.Input != 111 || sum.Prev.Replies != 1 || sum.Prev.Tokens.Input != 1 {
		t.Errorf("summary %+v", sum)
	}
	if len(sum.Buckets) != 24 || sum.Buckets[23] != 18 || sum.Buckets[22] != 153 || sum.Counts[21] != 1 {
		t.Errorf("buckets %v counts %v", sum.Buckets, sum.Counts)
	}
	if len(sum.Models) != 2 || sum.Models[0].Name != "claude-haiku" || len(sum.Projects) != 2 || sum.Projects[0].Name != "/x" || sum.Projects[0].Tokens != 171 {
		t.Errorf("models %+v projects %+v", sum.Models, sum.Projects)
	}
	if len(sum.Recent) != 3 || sum.Recent[0].Profile != "a" || sum.Recent[1].Profile != "b" || sum.Recent[2].Project != "/y" {
		t.Errorf("recent %+v", sum.Recent)
	}
}

func TestMergeStorage(t *testing.T) {
	st := MergeStorage([]Storage{
		{Total: 30, ByType: []Entry{{Name: "projects", Bytes: 20}, {Name: "history.jsonl", Bytes: 10}}, ByProject: []Entry{{Name: "-x", Bytes: 20}}},
		{Total: 25, ByType: []Entry{{Name: "projects", Bytes: 25}}, ByProject: []Entry{{Name: "-y", Bytes: 5}, {Name: "-x", Bytes: 20, Path: "/x"}}},
	})
	if st.Total != 55 || len(st.ByType) != 2 || st.ByType[0].Bytes != 45 {
		t.Errorf("by type %+v", st)
	}
	if len(st.ByProject) != 2 || st.ByProject[0].Name != "-x" || st.ByProject[0].Bytes != 40 || st.ByProject[0].Path != "/x" {
		t.Errorf("by project %+v", st.ByProject)
	}
}
