// Package keychain wraps the macOS `security` CLI to store the hush master
// age identity. The identity never lands on disk as a plaintext file; it lives
// in the login keychain under a fixed service name.
package keychain

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
)

// ErrNotFound is returned when the requested keychain item does not exist yet.
var ErrNotFound = errors.New("keychain: item not found")

const service = "hush"

// Get returns the secret stored under account, or ErrNotFound if absent.
func Get(account string) (string, error) {
	cmd := exec.Command("security", "find-generic-password",
		"-s", service, "-a", account, "-w")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		// security exits non-zero (44) when the item is missing.
		if strings.Contains(errb.String(), "could not be found") {
			return "", ErrNotFound
		}
		return "", err
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// Set stores secret under account, replacing any existing value (-U).
func Set(account, secret string) error {
	cmd := exec.Command("security", "add-generic-password",
		"-s", service, "-a", account, "-w", secret, "-U")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return errors.New("keychain: " + strings.TrimSpace(errb.String()))
	}
	return nil
}
