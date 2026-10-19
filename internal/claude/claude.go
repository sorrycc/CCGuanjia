// Package claude knows where Claude Code keeps its state: the default
// config folder ~/.claude with ~/.claude.json beside it, and the folders
// that CLAUDE_CONFIG_DIR names, each holding its own .claude.json, with a
// sign-in of its own in the Keychain. A folder is a profile. The package
// reads a profile's account from the local files only, links the items
// profiles share to ~/.claude, and signs a profile out.
//
// It never reads a credential: the Keychain is asked whether an item is
// there, as `security find-generic-password` without -w answers, and
// deleting one needs no secret either.
package claude

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"

	"ccguanjia/internal/jsonedit"
)

// DefaultName is the name of the profile of ~/.claude, which Claude Code
// uses when CLAUDE_CONFIG_DIR is not set.
const DefaultName = "default"

// SharedItems are what a profile may link to ~/.claude: the user's own
// configuration, which does not depend on the account. The sign-in,
// .claude.json and the history never are.
var SharedItems = []string{
	"skills",
	"plugins",
	"hooks",
	"CLAUDE.md",
	"settings.json",
	"statusline-command.js",
	"commands",
	"agents",
}

// Env is where Claude Code's state is: the home folder, and the Keychain,
// nil where there is none (Linux and Windows keep the sign-in in
// .credentials.json).
type Env struct {
	Home     string
	Keychain Keychain
	// Processes lists the running processes of Claude Code, nil for
	// Running's.
	Processes func() []Process
}

// Running lists the running processes of Claude Code.
func (e *Env) Running() []Process {
	if e.Processes != nil {
		return e.Processes()
	}
	return Running()
}

// MainDir is ~/.claude.
func (e *Env) MainDir() string { return filepath.Join(e.Home, ".claude") }

// DirFor is the folder of the profile name.
func (e *Env) DirFor(name string) string {
	if name == DefaultName {
		return e.MainDir()
	}
	return filepath.Join(e.Home, ".claude-"+name)
}

// Profile is a config folder of Claude Code.
type Profile struct {
	Name string
	Dir  string
	// Default is the profile of ~/.claude.
	Default bool
	// ConfigFile is its .claude.json: ~/.claude.json for the default
	// profile, inside the folder for the others.
	ConfigFile string
	// KeychainService is the Keychain item of its sign-in.
	KeychainService string
	// Account is the account of its .claude.json, nil when it has none.
	Account *Account
	// SignedIn reports whether its sign-in is stored, in the Keychain or
	// in .credentials.json.
	SignedIn bool
	// Links are the states of the shared items in it.
	Links []Link
}

// Account is what .claude.json says of the account signed in. It comes
// from the local file only and says nothing of whether the sign-in still
// works.
type Account struct {
	Email   string
	Name    string
	Org     string
	OrgType string
	Plan    string
}

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ValidName reports whether name can name a new profile: lowercase
// letters, digits, - and _, as a shell function's name takes them.
func ValidName(name string) error {
	if name == DefaultName {
		return fmt.Errorf("%q is the name of ~/.claude", name)
	}
	if !validName.MatchString(name) || len(name) > 40 {
		return fmt.Errorf("%q: use lowercase letters, digits, - and _", name)
	}
	return nil
}

// KeychainService is the Keychain item Claude Code keeps the sign-in of
// dir in: "Claude Code-credentials" for ~/.claude, and for a folder that
// CLAUDE_CONFIG_DIR names, the first 8 hex digits of the SHA-256 of its
// path after it.
func KeychainService(dir string, isDefault bool) string {
	const base = "Claude Code-credentials"
	if isDefault {
		return base
	}
	sum := sha256.Sum256([]byte(norm.NFC.String(dir)))
	return base + "-" + hex.EncodeToString(sum[:])[:8]
}

// Discover returns the profiles: ~/.claude first, then each ~/.claude-*
// folder by name.
func (e *Env) Discover() ([]*Profile, error) {
	profiles := []*Profile{e.Load(DefaultName)}
	entries, err := os.ReadDir(e.Home)
	if err != nil {
		return profiles, err
	}
	var names []string
	for _, ent := range entries {
		name, ok := strings.CutPrefix(ent.Name(), ".claude-")
		if !ok || name == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(e.Home, ent.Name())); err == nil && fi.IsDir() {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		profiles = append(profiles, e.Load(name))
	}
	return profiles, nil
}

// Load reads the profile name, which need not exist yet.
func (e *Env) Load(name string) *Profile {
	p := &Profile{Name: name, Dir: e.DirFor(name), Default: name == DefaultName}
	if p.Default {
		p.ConfigFile = filepath.Join(e.Home, ".claude.json")
	} else {
		p.ConfigFile = filepath.Join(p.Dir, ".claude.json")
	}
	p.KeychainService = KeychainService(p.Dir, p.Default)
	p.Account = readAccount(p.ConfigFile)
	p.SignedIn = e.signedIn(p)
	p.Links = e.links(p)
	return p
}

// Exists reports whether the profile's folder is there.
func (p *Profile) Exists() bool {
	fi, err := os.Stat(p.Dir)
	return err == nil && fi.IsDir()
}

// CredentialsFile is the file Claude Code keeps the sign-in in where there
// is no Keychain.
func (p *Profile) CredentialsFile() string { return filepath.Join(p.Dir, ".credentials.json") }

func (e *Env) signedIn(p *Profile) bool {
	if fileExists(p.CredentialsFile()) {
		return true
	}
	if e.Keychain == nil {
		return false
	}
	_, ok, _ := e.Keychain.Find(p.KeychainService)
	return ok
}

func readAccount(path string) *Account {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	o, err := jsonedit.Parse(data)
	if err != nil {
		return nil
	}
	var raw struct {
		Email     string `json:"emailAddress"`
		Display   string `json:"displayName"`
		Full      string `json:"fullName"`
		Org       string `json:"organizationName"`
		OrgType   string `json:"organizationType"`
		OrgTier   string `json:"organizationRateLimitTier"`
		UserTier  string `json:"userRateLimitTier"`
		SeatTier  string `json:"seatTier"`
		Billing   string `json:"billingType"`
		AccountID string `json:"accountUuid"`
	}
	if ok, err := o.Get("oauthAccount", &raw); !ok || err != nil || raw.AccountID == "" && raw.Email == "" {
		return nil
	}
	name := raw.Display
	if name == "" {
		name = raw.Full
	}
	return &Account{
		Email:   raw.Email,
		Name:    name,
		Org:     raw.Org,
		OrgType: raw.OrgType,
		Plan:    planOf(raw.OrgType, raw.OrgTier+" "+raw.UserTier+" "+raw.SeatTier),
	}
}

// planOf names the plan of an organization's type, as Claude Code's own
// /status does, with Max's tier when the rate limit tier tells it.
func planOf(orgType, tiers string) string {
	switch orgType {
	case "claude_max":
		switch {
		case strings.Contains(tiers, "20x"):
			return "Max 20x"
		case strings.Contains(tiers, "5x"):
			return "Max 5x"
		}
		return "Max"
	case "claude_pro":
		return "Pro"
	case "claude_team":
		return "Team"
	case "claude_enterprise":
		return "Enterprise"
	case "":
		return ""
	}
	return strings.TrimPrefix(orgType, "claude_")
}

// LinkState is what a shared item of a profile is.
type LinkState int

const (
	// Unlinked: nothing is there.
	Unlinked LinkState = iota
	// Linked: a link to the item of ~/.claude.
	Linked
	// LinkedElsewhere: a link to somewhere else.
	LinkedElsewhere
	// Own: a file or folder of the profile's own, which linking would
	// replace, so it never does.
	Own
)

// Link is the state of a shared item in a profile.
type Link struct {
	Item  string
	State LinkState
	// Target is where a link points.
	Target string
	// Source reports whether ~/.claude has the item to link to.
	Source bool
}

func (e *Env) links(p *Profile) []Link {
	if p.Default {
		return nil
	}
	links := make([]Link, 0, len(SharedItems))
	for _, item := range SharedItems {
		l := Link{Item: item, Source: exists(filepath.Join(e.MainDir(), item))}
		dest := filepath.Join(p.Dir, item)
		fi, err := os.Lstat(dest)
		switch {
		case err != nil:
		case fi.Mode()&os.ModeSymlink != 0:
			l.Target, _ = os.Readlink(dest)
			l.State = LinkedElsewhere
			if sameFile(resolve(p.Dir, l.Target), filepath.Join(e.MainDir(), item)) {
				l.State = Linked
			}
		default:
			l.State = Own
		}
		links = append(links, l)
	}
	return links
}

// LinkOf returns the state of item in the profile.
func (p *Profile) LinkOf(item string) Link {
	for _, l := range p.Links {
		if l.Item == item {
			return l
		}
	}
	return Link{Item: item}
}

func resolve(dir, target string) string {
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Join(dir, target)
}

func sameFile(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	return err1 == nil && err2 == nil && ra == rb
}

// ErrOwn is the error of linking over a profile's own file or folder.
var ErrOwn = errors.New("the profile has its own, which linking would replace")

// Create makes the folder of a new profile, and links items into it.
func (e *Env) Create(name string, items []string) (*Profile, error) {
	if err := ValidName(name); err != nil {
		return nil, err
	}
	dir := e.DirFor(name)
	if exists(dir) {
		return nil, fmt.Errorf("%s exists already", dir)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	p := e.Load(name)
	for _, item := range items {
		if !exists(filepath.Join(e.MainDir(), item)) {
			continue
		}
		if err := e.Link(p, item); err != nil {
			return p, err
		}
	}
	return e.Load(name), nil
}

// Link links item of the profile to ~/.claude, replacing a link but never
// a file or folder of its own.
func (e *Env) Link(p *Profile, item string) error {
	if p.Default {
		return errors.New("~/.claude is what the others link to")
	}
	if !slices.Contains(SharedItems, item) {
		return fmt.Errorf("%q is not an item profiles share", item)
	}
	src := filepath.Join(e.MainDir(), item)
	if !exists(src) {
		return fmt.Errorf("~/.claude has no %s", item)
	}
	dest := filepath.Join(p.Dir, item)
	if fi, err := os.Lstat(dest); err == nil {
		if fi.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%s: %w", item, ErrOwn)
		}
		if err := os.Remove(dest); err != nil {
			return err
		}
	}
	return os.Symlink(src, dest)
}

// Unlink removes the link of item from the profile. It removes only a
// link, never a file or folder of the profile's own.
func (e *Env) Unlink(p *Profile, item string) error {
	dest := filepath.Join(p.Dir, item)
	fi, err := os.Lstat(dest)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s is not a link", item)
	}
	return os.Remove(dest)
}

// Remove deletes a profile's folder, with its history, and its sign-in.
func (e *Env) Remove(p *Profile) error {
	if p.Default {
		return errors.New("~/.claude is not removed")
	}
	if filepath.Dir(p.Dir) != filepath.Clean(e.Home) || !strings.HasPrefix(filepath.Base(p.Dir), ".claude-") {
		return fmt.Errorf("%s is not a profile's folder", p.Dir)
	}
	if e.Keychain != nil {
		if acct, ok, _ := e.Keychain.Find(p.KeychainService); ok {
			if err := e.Keychain.Delete(p.KeychainService, acct); err != nil {
				return fmt.Errorf("the Keychain item: %w", err)
			}
		}
	}
	return os.RemoveAll(p.Dir)
}

// MCPServers returns the names of the MCP servers of the profile's
// .claude.json, sorted.
func (p *Profile) MCPServers() []string {
	o, err := readObject(p.ConfigFile)
	if err != nil {
		return nil
	}
	servers, err := subObject(o, "mcpServers")
	if err != nil || servers == nil {
		return nil
	}
	names := servers.Keys()
	sort.Strings(names)
	return names
}

// CopyMCP adds the MCP servers of from's .claude.json to to's, leaving the
// rest of the file and the servers to has of the same name as they are. It
// returns the names of the servers added.
func CopyMCP(from, to *Profile) ([]string, error) {
	src, err := readObject(from.ConfigFile)
	if err != nil {
		return nil, err
	}
	srcServers, err := subObject(src, "mcpServers")
	if err != nil {
		return nil, err
	}
	if srcServers == nil || len(srcServers.Keys()) == 0 {
		return nil, nil
	}
	data, err := os.ReadFile(to.ConfigFile)
	if errors.Is(err, os.ErrNotExist) {
		data = []byte("{}")
	} else if err != nil {
		return nil, err
	}
	dst, err := jsonedit.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", to.ConfigFile, err)
	}
	dstServers, err := subObject(dst, "mcpServers")
	if err != nil {
		return nil, err
	}
	if dstServers == nil {
		dstServers, _ = jsonedit.Parse([]byte("{}"))
	}
	var added []string
	for _, name := range srcServers.Keys() {
		if dstServers.Has(name) {
			continue
		}
		dstServers.SetRaw(name, srcServers.Raw(name))
		added = append(added, name)
	}
	if len(added) == 0 {
		return nil, nil
	}
	raw, err := dstServers.Marshal("")
	if err != nil {
		return nil, err
	}
	dst.SetRaw("mcpServers", raw)
	return added, WriteObject(to.ConfigFile, dst, jsonedit.Indent(data))
}

func readObject(path string) (*jsonedit.Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	o, err := jsonedit.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return o, nil
}

func subObject(o *jsonedit.Object, key string) (*jsonedit.Object, error) {
	raw := o.Raw(key)
	if raw == nil || string(raw) == "null" {
		return nil, nil
	}
	return jsonedit.Parse(raw)
}

// WriteObject writes o to path through a file beside it, which takes the
// place of the old one at once, with the old one's permissions (0600 for a
// new one, as Claude Code makes them).
func WriteObject(path string, o *jsonedit.Object, indent string) error {
	out, err := o.Marshal(indent)
	if err != nil {
		return err
	}
	return WriteFile(path, out)
}

// WriteFile writes data to path through a file beside it, keeping the
// mode of the file it replaces. A path that is a link is written where
// the link points, so that a shared settings.json stays shared.
func WriteFile(path string, data []byte) error {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".ccguanjia-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}
