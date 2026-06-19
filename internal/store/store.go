// Package store manages the single global encrypted secret file.
//
// Layout on disk:
//
//	~/.config/hush/store.age   age-encrypted JSON
//
// Cleartext JSON is namespaced project -> profile -> key -> value, so two
// projects on the same branch name never collide. The age identity used to
// encrypt/decrypt lives in the macOS keychain (see internal/keychain); it is
// generated on first use and never written to disk as plaintext.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"filippo.io/age"
	"github.com/allen-hsu/hush/internal/keychain"
)

// keychainAccount is the keychain item that holds the age secret key.
const keychainAccount = "master-identity"

// Data is the cleartext model: project -> profile -> key -> value.
type Data map[string]map[string]map[string]string

// Store is a handle to the on-disk encrypted store plus the identity to read it.
type Store struct {
	path     string
	identity *age.X25519Identity
}

// Path returns the store file path, honoring HUSH_STORE for tests/overrides.
func Path() string {
	if p := os.Getenv("HUSH_STORE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "hush", "store.age")
}

// Open loads the master identity from the keychain, generating and persisting a
// fresh one on first use, and returns a Store ready to Load/Save.
func Open() (*Store, error) {
	id, err := loadOrCreateIdentity()
	if err != nil {
		return nil, err
	}
	return &Store{path: Path(), identity: id}, nil
}

func loadOrCreateIdentity() (*age.X25519Identity, error) {
	sec, err := keychain.Get(keychainAccount)
	if errors.Is(err, keychain.ErrNotFound) {
		id, gerr := age.GenerateX25519Identity()
		if gerr != nil {
			return nil, gerr
		}
		if serr := keychain.Set(keychainAccount, id.String()); serr != nil {
			return nil, serr
		}
		return id, nil
	}
	if err != nil {
		return nil, err
	}
	return age.ParseX25519Identity(sec)
}

// Load decrypts and returns the store contents (empty Data if the file is absent).
func (s *Store) Load() (Data, error) {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Data{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := age.Decrypt(f, s.identity)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return Data{}, nil
	}
	var d Data
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	if d == nil {
		d = Data{}
	}
	return d, nil
}

// Save encrypts and atomically writes the store contents.
func (s *Store) Save(d Data) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, s.identity.Recipient())
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
