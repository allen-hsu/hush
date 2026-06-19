package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// keysArrayRe matches a `keys = [ ... ]` assignment, single- or multi-line.
// The value list never contains `]` (keys are identifiers), so a negated class
// safely spans newlines.
var keysArrayRe = regexp.MustCompile(`(?m)^[ \t]*keys[ \t]*=[ \t]*\[[^\]]*\]`)

// WriteKeys updates ONLY the `keys = [...]` array in the dir's .hush.toml,
// leaving every other line — comments, field order, project/profile/shims —
// byte-for-byte intact. hush never owns any field but `keys`, so this is the
// right surgical edit rather than a lossy full re-marshal.
func WriteKeys(dir string, keys []string) error {
	path := filepath.Join(dir, FileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	uniq := dedupeSorted(keys)
	line := "keys = " + formatArray(uniq)

	var out string
	if keysArrayRe.Match(raw) {
		out = keysArrayRe.ReplaceAllString(string(raw), line)
	} else {
		// No keys array yet — append one, keeping a trailing newline tidy.
		out = strings.TrimRight(string(raw), "\n") + "\n" + line + "\n"
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

func dedupeSorted(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func formatArray(ss []string) string {
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
