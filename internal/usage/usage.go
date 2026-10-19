// Package usage reads what a profile's transcripts say of the tokens it
// used, and what its folder takes on disk. It reads the local files only:
// the transcripts in projects/, one line per message, whose replies carry
// the tokens of their request.
package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Tokens are the tokens of requests.
type Tokens struct {
	Input, Output, CacheWrite, CacheRead int64
}

// Total is every token.
func (t Tokens) Total() int64 { return t.Input + t.Output + t.CacheWrite + t.CacheRead }

func (t *Tokens) add(u Tokens) {
	t.Input += u.Input
	t.Output += u.Output
	t.CacheWrite += u.CacheWrite
	t.CacheRead += u.CacheRead
}

// entry is a reply of a transcript.
type entry struct {
	id      string
	at      int64 // Unix seconds
	model   string
	project string
	session string
	tokens  Tokens
}

type fileCache struct {
	size    int64
	mod     time.Time
	entries []entry
}

// Scanner reads transcripts and keeps what it read of each file until the
// file changes. It is safe for one goroutine at a time.
type Scanner struct {
	mu    sync.Mutex
	files map[string]*fileCache
}

// NewScanner returns an empty scanner.
func NewScanner() *Scanner { return &Scanner{files: map[string]*fileCache{}} }

// Summary is the usage of a window of time.
type Summary struct {
	Since, Until time.Time
	Tokens       Tokens
	// Replies counts the replies, Sessions the sessions with one.
	Replies  int
	Sessions int
	// Buckets are the tokens of equal spans of the window, oldest first,
	// and Counts the replies of each.
	Buckets []int64
	Counts  []int64
	// Models and Projects are the tokens of each, most first.
	Models   []Share
	Projects []Share
	// Hours counts the replies of each hour of the week, in local time:
	// Hours[weekday][hour], Sunday first, as time.Weekday has them.
	Hours [7][24]int64
	// Recent are the sessions of the window, the latest active first, at
	// most RecentSessions of them.
	Recent []Session
}

// RecentSessions is how many sessions Summary.Recent keeps.
const RecentSessions = 8

// Session is a conversation of a transcript, over the window.
type Session struct {
	ID string
	// Project is the folder it ran in.
	Project     string
	First, Last time.Time
	Replies     int
	Tokens      int64
	// Model is the model of most of its tokens.
	Model string
}

// Share is the tokens of a model or a project.
type Share struct {
	Name   string
	Tokens int64
}

// Scan reads the transcripts of dir (a profile's folder) that changed
// since `since`, and sums the replies from then until now in n buckets.
func (s *Scanner) Scan(dir string, since, now time.Time, n int) Summary {
	s.mu.Lock()
	defer s.mu.Unlock()
	root := filepath.Join(dir, "projects")
	seenFiles := map[string]bool{}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		fi, err := d.Info()
		if err != nil || fi.ModTime().Before(since) {
			return nil
		}
		seenFiles[path] = true
		if c := s.files[path]; c != nil && c.size == fi.Size() && c.mod.Equal(fi.ModTime()) {
			return nil
		}
		s.files[path] = &fileCache{size: fi.Size(), mod: fi.ModTime(), entries: readFile(path)}
		return nil
	})
	for path := range s.files {
		if strings.HasPrefix(path, root+string(filepath.Separator)) && !seenFiles[path] {
			delete(s.files, path)
		}
	}

	sum := Summary{Since: since, Until: now, Buckets: make([]int64, max(n, 1)), Counts: make([]int64, max(n, 1))}
	span := now.Sub(since).Seconds() / float64(len(sum.Buckets))
	seen := map[string]bool{}
	sessions := map[string]*sessionAcc{}
	models := map[string]int64{}
	projects := map[string]int64{}
	from, to := since.Unix(), now.Unix()
	for path, c := range s.files {
		if !strings.HasPrefix(path, root+string(filepath.Separator)) {
			continue
		}
		for _, e := range c.entries {
			if e.at < from || e.at > to {
				continue
			}
			if e.id != "" {
				if seen[e.id] {
					continue
				}
				seen[e.id] = true
			}
			sum.Tokens.add(e.tokens)
			sum.Replies++
			sa := sessions[e.session]
			if sa == nil {
				sa = &sessionAcc{Session: Session{ID: e.session, Project: e.project}, first: e.at, last: e.at, models: map[string]int64{}}
				sessions[e.session] = sa
			}
			sa.add(e)
			t := time.Unix(e.at, 0)
			sum.Hours[t.Weekday()][t.Hour()]++
			models[e.model] += e.tokens.Total()
			projects[e.project] += e.tokens.Total()
			i := int(float64(e.at-from) / span)
			i = min(max(i, 0), len(sum.Buckets)-1)
			sum.Buckets[i] += e.tokens.Total()
			sum.Counts[i]++
		}
	}
	sum.Sessions = len(sessions)
	sum.Recent = recent(sessions, RecentSessions)
	sum.Models = shares(models)
	sum.Projects = shares(projects)
	return sum
}

// sessionAcc sums the replies of a session.
type sessionAcc struct {
	Session
	first, last int64
	models      map[string]int64
}

func (s *sessionAcc) add(e entry) {
	s.Replies++
	s.Tokens += e.tokens.Total()
	s.models[e.model] += e.tokens.Total()
	s.first, s.last = min(s.first, e.at), max(s.last, e.at)
	if e.at == s.last && e.project != "" {
		s.Project = e.project
	}
}

// recent are the n sessions active last.
func recent(m map[string]*sessionAcc, n int) []Session {
	out := make([]Session, 0, len(m))
	for _, sa := range m {
		s := sa.Session
		s.First, s.Last = time.Unix(sa.first, 0), time.Unix(sa.last, 0)
		if ms := shares(sa.models); len(ms) > 0 {
			s.Model = ms[0].Name
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Last.Equal(out[j].Last) {
			return out[i].Last.After(out[j].Last)
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func shares(m map[string]int64) []Share {
	out := make([]Share, 0, len(m))
	for k, v := range m {
		out = append(out, Share{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tokens != out[j].Tokens {
			return out[i].Tokens > out[j].Tokens
		}
		return out[i].Name < out[j].Name
	})
	return out
}

var (
	assistantMark = []byte(`"assistant"`)
	usageMark     = []byte(`"usage"`)
)

// readFile reads the replies of a transcript. The lines that say no usage
// are passed over without being decoded.
func readFile(path string) []entry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 256*1024), 64*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.Contains(line, usageMark) || !bytes.Contains(line, assistantMark) {
			continue
		}
		var v struct {
			Type      string    `json:"type"`
			Timestamp time.Time `json:"timestamp"`
			RequestID string    `json:"requestId"`
			Session   string    `json:"sessionId"`
			Cwd       string    `json:"cwd"`
			Message   struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage *struct {
					Input      int64 `json:"input_tokens"`
					Output     int64 `json:"output_tokens"`
					CacheWrite int64 `json:"cache_creation_input_tokens"`
					CacheRead  int64 `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &v) != nil || v.Type != "assistant" || v.Message.Usage == nil || v.Message.Model == "<synthetic>" {
			continue
		}
		u := v.Message.Usage
		id := ""
		if v.Message.ID != "" {
			id = v.Message.ID + ":" + v.RequestID
		}
		out = append(out, entry{
			id:      id,
			at:      v.Timestamp.Unix(),
			model:   v.Message.Model,
			project: v.Cwd,
			session: v.Session,
			tokens:  Tokens{u.Input, u.Output, u.CacheWrite, u.CacheRead},
		})
	}
	return out
}

// Entry is a part of a folder and its size.
type Entry struct {
	Name  string
	Bytes int64
	// Path is a project's folder, as its transcripts tell it.
	Path string
}

// Storage is what a profile's folder takes.
type Storage struct {
	Total int64
	// ByType are its files and folders, the links left out, most first.
	ByType []Entry
	// ByProject are the folders of projects/, most first.
	ByProject []Entry
}

// MeasureStorage measures dir, not following links.
func MeasureStorage(dir string) Storage {
	var st Storage
	ents, err := os.ReadDir(dir)
	if err != nil {
		return st
	}
	for _, e := range ents {
		if e.Type()&fs.ModeSymlink != 0 {
			continue
		}
		n := treeSize(filepath.Join(dir, e.Name()))
		st.Total += n
		st.ByType = append(st.ByType, Entry{Name: e.Name(), Bytes: n})
	}
	sortEntries(st.ByType)
	projects := filepath.Join(dir, "projects")
	if ents, err := os.ReadDir(projects); err == nil {
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			path := filepath.Join(projects, e.Name())
			st.ByProject = append(st.ByProject, Entry{Name: e.Name(), Bytes: treeSize(path), Path: projectPath(path)})
		}
	}
	sortEntries(st.ByProject)
	return st
}

func sortEntries(es []Entry) {
	sort.Slice(es, func(i, j int) bool {
		if es[i].Bytes != es[j].Bytes {
			return es[i].Bytes > es[j].Bytes
		}
		return es[i].Name < es[j].Name
	})
}

func treeSize(root string) int64 {
	var n int64
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

// projectPath is the working folder a project's transcripts were written
// in: the name of its folder in projects/ only replaces its slashes, so
// is no way back to it.
func projectPath(dir string) string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		if cwd := firstCwd(filepath.Join(dir, e.Name())); cwd != "" {
			return cwd
		}
	}
	return ""
}

var cwdMark = []byte(`"cwd":"`)

func firstCwd(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 256*1024), 64*1024*1024)
	for i := 0; i < 50 && sc.Scan(); i++ {
		line := sc.Bytes()
		if !bytes.Contains(line, cwdMark) {
			continue
		}
		var v struct {
			Cwd string `json:"cwd"`
		}
		if json.Unmarshal(line, &v) == nil && v.Cwd != "" {
			return v.Cwd
		}
	}
	return ""
}
