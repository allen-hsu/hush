package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSecureTempDir_RamBackedOnMacOS(t *testing.T) {
	dir, cleanup, ram, err := secureTempDir("hush-test-")
	if err != nil {
		t.Fatalf("secureTempDir: %v", err)
	}
	defer cleanup()

	// Write a file and read it back.
	f := filepath.Join(dir, "x.env")
	if err := os.WriteFile(f, []byte("K=v\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if b, err := os.ReadFile(f); err != nil || string(b) != "K=v\n" {
		t.Fatalf("readback failed: %q err=%v", b, err)
	}

	if runtime.GOOS == "darwin" {
		if !ram {
			// Some sandboxed CI environments forbid hdiutil; the disk fallback is
			// still correct, so skip rather than fail.
			t.Skip("RAM disk unavailable in this environment; using disk fallback")
		}
		if !strings.HasPrefix(dir, "/Volumes/") {
			t.Errorf("expected RAM disk under /Volumes, got %s", dir)
		}
	}

	// After cleanup the mount/dir must be gone.
	cleanup()
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Errorf("expected %s gone after cleanup, stat err=%v", f, err)
	}
}
