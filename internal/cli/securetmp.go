package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// secureTempDir returns a directory for short-lived plaintext (the edit flow).
//
// On macOS it creates a RAM-backed volume via hdiutil so plaintext NEVER touches
// persistent storage — important because secure-deletion is unreliable on
// APFS/SSD (copy-on-write means overwriting a file doesn't overwrite its blocks).
// The whole volume is detached on cleanup and vanishes with the RAM.
//
// If a RAM disk can't be created (non-macOS, hdiutil missing, sandbox), it falls
// back to a 0700 dir under TMPDIR and reports ramBacked=false so the caller can
// warn that plaintext briefly touches disk.
func secureTempDir(label string) (dir string, cleanup func(), ramBacked bool, err error) {
	if runtime.GOOS == "darwin" {
		if d, c, ok := ramDisk(label); ok {
			return d, c, true, nil
		}
	}
	d, e := os.MkdirTemp("", label)
	if e != nil {
		return "", func() {}, false, e
	}
	_ = os.Chmod(d, 0o700)
	return d, func() { _ = os.RemoveAll(d) }, false, nil
}

// ramDisk attaches a small (~8 MiB) RAM device and formats it as a volume,
// returning its mount point and a detach cleanup. No sudo required.
func ramDisk(label string) (mount string, cleanup func(), ok bool) {
	// 16384 sectors * 512 B = 8 MiB — ample for an env file.
	out, err := exec.Command("hdiutil", "attach", "-nomount", "ram://16384").Output()
	if err != nil {
		return "", nil, false
	}
	dev := strings.Fields(strings.TrimSpace(string(out)))
	if len(dev) == 0 {
		return "", nil, false
	}
	node := dev[0]
	detach := func() { _ = exec.Command("hdiutil", "detach", "-force", node).Run() }

	name := label + strconv.Itoa(os.Getpid())
	// diskutil erasevolume formats AND mounts at /Volumes/<name> without sudo.
	if err := exec.Command("diskutil", "erasevolume", "HFS+", name, node).Run(); err != nil {
		detach()
		return "", nil, false
	}
	mount = filepath.Join("/Volumes", name)
	_ = os.Chmod(mount, 0o700)
	return mount, detach, true
}
