package cli

import (
	"strings"
	"testing"
)

func TestParseDotenv(t *testing.T) {
	in := `# a comment
export A=1
B="two words"
C='single'
D=val # inline comment
EMPTY=

  # indented comment
E = spaced
`
	m, err := parseDotenv(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"A":     "1",
		"B":     "two words",
		"C":     "single",
		"D":     "val",
		"EMPTY": "",
		"E":     "spaced",
	}
	for k, v := range want {
		if got, ok := m[k]; !ok || got != v {
			t.Errorf("%s: want %q, got %q (present=%v)", k, v, got, ok)
		}
	}
	if len(m) != len(want) {
		t.Errorf("unexpected extra keys: %v", m)
	}
}
