package resolve_test

import (
	"testing"

	"github.com/allen-hsu/hush/internal/config"
	"github.com/allen-hsu/hush/internal/resolve"
	"github.com/allen-hsu/hush/internal/store"
)

func TestValues_ExtendsChainAndMissing(t *testing.T) {
	cfg := &config.Config{Keys: []string{"A", "B", "C"}, Extends: "base"}
	ctx := resolve.Context{Project: "p", Profile: "feat"}
	data := store.Data{"p": {
		"feat": {"A": "1"},                  // A resolves from the active profile
		"base": {"B": "2", "A": "shadowed"}, // B from base; A in feat wins
	}}

	vals, missing := resolve.Values(cfg, ctx, data)

	if vals["A"] != "1" {
		t.Errorf("A: want active-profile value %q, got %q", "1", vals["A"])
	}
	if vals["B"] != "2" {
		t.Errorf("B: want base value %q, got %q", "2", vals["B"])
	}
	if len(missing) != 1 || missing[0] != "C" {
		t.Errorf("want missing=[C], got %v", missing)
	}
}

func TestResolve_FixedProfile(t *testing.T) {
	cfg := &config.Config{Project: "p", Profile: "fixed:prod"}
	ctx, err := resolve.Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Project != "p" || ctx.Profile != "prod" {
		t.Errorf("got project=%q profile=%q", ctx.Project, ctx.Profile)
	}
	if ctx.Warning != "" {
		t.Errorf("fixed profile should not warn, got %q", ctx.Warning)
	}
}
