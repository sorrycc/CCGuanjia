package claude

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// Keychain finds and deletes the items that keep Claude Code's sign-ins.
// Neither reads an item's secret.
type Keychain interface {
	// Find reports whether an item of service is there, with its
	// account.
	Find(service string) (account string, ok bool, err error)
	// Delete deletes the item of service and account.
	Delete(service, account string) error
}

// SystemKeychain is the login Keychain of macOS, through
// /usr/bin/security, and nil elsewhere.
func SystemKeychain() Keychain {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return securityTool{}
}

type securityTool struct{}

var acctAttr = regexp.MustCompile(`"acct"<blob>="([^"]*)"`)

// Find runs `security find-generic-password -s service`: without -w it
// prints the item's attributes only, so the secret never leaves the
// Keychain, and macOS asks nothing.
func (securityTool) Find(service string) (string, bool, error) {
	var out, errOut bytes.Buffer
	cmd := exec.Command("/usr/bin/security", "find-generic-password", "-s", service)
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		// 44 is errSecItemNotFound.
		if errors.As(err, &exit) && exit.ExitCode() == 44 {
			return "", false, nil
		}
		return "", false, fmt.Errorf("security: %s", strings.TrimSpace(errOut.String()))
	}
	acct := ""
	if m := acctAttr.FindStringSubmatch(out.String()); m != nil {
		acct = m[1]
	}
	return acct, true, nil
}

func (securityTool) Delete(service, account string) error {
	args := []string{"delete-generic-password", "-s", service}
	if account != "" {
		args = append(args, "-a", account)
	}
	out, err := exec.Command("/usr/bin/security", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("security: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// FakeKeychain is a Keychain in memory, for tests: services to accounts.
type FakeKeychain map[string]string

func (k FakeKeychain) Find(service string) (string, bool, error) {
	acct, ok := k[service]
	return acct, ok, nil
}

func (k FakeKeychain) Delete(service, account string) error {
	if _, ok := k[service]; !ok {
		return errors.New("no such item")
	}
	delete(k, service)
	return nil
}
