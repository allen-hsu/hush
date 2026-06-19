package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

// newTestStore builds a Store with a throwaway identity, bypassing the keychain
// so tests are hermetic.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	return &Store{path: filepath.Join(t.TempDir(), "store.age"), identity: id}
}

func TestLoad_MissingFileIsEmpty(t *testing.T) {
	s := newTestStore(t)
	d, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(d) != 0 {
		t.Errorf("expected empty Data for missing file, got %v", d)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	in := Data{
		"proj": {
			"main": {"K": "v", "K2": "v2"},
			"base": {"X": "y"},
		},
		"other": {"main": {"Z": "z"}},
	}
	if err := s.Save(in); err != nil {
		t.Fatal(err)
	}
	out, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if out["proj"]["main"]["K"] != "v" ||
		out["proj"]["base"]["X"] != "y" ||
		out["other"]["main"]["Z"] != "z" {
		t.Errorf("round-trip mismatch: %#v", out)
	}
}

func TestSave_EncryptsAtRest(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(Data{"p": {"prof": {"SECRET": "sk_live_supersecret"}}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("sk_live_supersecret")) {
		t.Error("plaintext secret found in store file at rest")
	}
	if !bytes.HasPrefix(raw, []byte("age-encryption.org")) {
		t.Error("store file is not age-encrypted")
	}
}

func TestLoad_WrongIdentityFails(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(Data{"p": {"q": {"K": "v"}}}); err != nil {
		t.Fatal(err)
	}
	// Open the same file with a different identity — must not decrypt.
	other, _ := age.GenerateX25519Identity()
	s2 := &Store{path: s.path, identity: other}
	if _, err := s2.Load(); err == nil {
		t.Error("expected decryption to fail with the wrong identity")
	}
}
