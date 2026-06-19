package cli

import "testing"

func TestHasAgentMarker(t *testing.T) {
	// Clear all known markers first so the environment is deterministic.
	for _, k := range agentEnvMarkers {
		t.Setenv(k, "")
	}
	if hasAgentMarker() {
		t.Fatal("no markers set, want false")
	}

	cases := []string{"CLAUDECODE", "CODEX_SANDBOX", "HUSH_AGENT"}
	for _, marker := range cases {
		t.Run(marker, func(t *testing.T) {
			for _, k := range agentEnvMarkers {
				t.Setenv(k, "")
			}
			t.Setenv(marker, "1")
			if !hasAgentMarker() {
				t.Errorf("%s set, want hasAgentMarker()=true", marker)
			}
		})
	}
}
