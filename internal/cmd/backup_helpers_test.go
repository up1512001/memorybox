package cmd

import (
	"testing"
)

func TestMergeExcludes_appendsNew(t *testing.T) {
	base := []string{"*.log", "tmp/"}
	extra := []string{"node_modules", "*.log", "dist/"}

	got := mergeExcludes(base, extra)

	want := []string{"*.log", "tmp/", "node_modules", "dist/"}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestMergeExcludes_emptyBase(t *testing.T) {
	got := mergeExcludes(nil, []string{"vendor", "dist"})
	if len(got) != 2 {
		t.Fatalf("expected 2 patterns, got %v", got)
	}
}

func TestMergeExcludes_emptyExtra(t *testing.T) {
	base := []string{"*.log"}
	got := mergeExcludes(base, nil)
	if len(got) != 1 || got[0] != "*.log" {
		t.Errorf("expected base unchanged, got %v", got)
	}
}

func TestMergeExcludes_noDuplicates(t *testing.T) {
	base := []string{"a", "b", "c"}
	got := mergeExcludes(base, []string{"a", "b", "c"})
	if len(got) != 3 {
		t.Errorf("expected 3 (no dupes), got %d: %v", len(got), got)
	}
}

func TestMergeExcludes_doesNotMutateBase(t *testing.T) {
	base := []string{"a"}
	_ = mergeExcludes(base, []string{"b"})
	if len(base) != 1 {
		t.Errorf("base was mutated: %v", base)
	}
}
