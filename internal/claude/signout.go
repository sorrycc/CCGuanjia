package claude

import (
	"errors"
	"fmt"
	"os"

	"ccguanjia/internal/jsonedit"
)

// IdentityKeys are the keys of .claude.json that signing out drops: what
// `claude /logout` clears, and the telemetry ID, the date of the first
// token and the API keys approved or rejected beside them.
var IdentityKeys = []string{"oauthAccount", "userID", "claudeCodeFirstTokenDate", "customApiKeyResponses"}

// SignOutPlan is what signing a profile out removes.
type SignOutPlan struct {
	// KeychainService and KeychainAccount name the Keychain item, ""
	// when there is none.
	KeychainService string
	KeychainAccount string
	// CredentialsFile is the .credentials.json, "" when there is none.
	CredentialsFile string
	// Keys are the identity keys .claude.json has.
	Keys []string
}

// Empty reports whether there is nothing to sign out of.
func (s SignOutPlan) Empty() bool {
	return s.KeychainService == "" && s.CredentialsFile == "" && len(s.Keys) == 0
}

// PlanSignOut finds what signing p out removes.
func (e *Env) PlanSignOut(p *Profile) SignOutPlan {
	var s SignOutPlan
	if e.Keychain != nil {
		if acct, ok, _ := e.Keychain.Find(p.KeychainService); ok {
			s.KeychainService, s.KeychainAccount = p.KeychainService, acct
		}
	}
	if fileExists(p.CredentialsFile()) {
		s.CredentialsFile = p.CredentialsFile()
	}
	if o, err := readObject(p.ConfigFile); err == nil {
		for _, k := range IdentityKeys {
			if o.Has(k) {
				s.Keys = append(s.Keys, k)
			}
		}
	}
	return s
}

// SignOut removes what s names: the Keychain item first, and on its
// failure nothing else, since a sign-out that clears the account but
// keeps the token is the worst of both. It returns what it removed. The
// token is not kept anywhere: signing in again restores it.
func (e *Env) SignOut(p *Profile, s SignOutPlan) ([]string, error) {
	var removed []string
	if s.KeychainService != "" {
		if e.Keychain == nil {
			return nil, errors.New("no Keychain")
		}
		if err := e.Keychain.Delete(s.KeychainService, s.KeychainAccount); err != nil {
			return nil, fmt.Errorf("the Keychain item: %w", err)
		}
		removed = append(removed, "Keychain: "+s.KeychainService)
	}
	if s.CredentialsFile != "" {
		if err := os.Remove(s.CredentialsFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		removed = append(removed, ".credentials.json")
	}
	if len(s.Keys) > 0 {
		data, err := os.ReadFile(p.ConfigFile)
		if err != nil {
			return removed, err
		}
		o, err := jsonedit.Parse(data)
		if err != nil {
			return removed, err
		}
		for _, k := range s.Keys {
			if o.Delete(k) {
				removed = append(removed, ".claude.json: "+k)
			}
		}
		// Two spaces, as Claude Code writes the file.
		if err := WriteObject(p.ConfigFile, o, "  "); err != nil {
			return removed, err
		}
	}
	return removed, nil
}
