package api

import (
	"strings"
	"testing"
)

func TestGenerateThreadID_Format(t *testing.T) {
	id := generateThreadID()
	parts := strings.Split(id, "-")
	if len(parts) != 3 {
		t.Errorf("expected 3 hyphen-separated words, got %q (%d parts)", id, len(parts))
	}
	for _, p := range parts {
		if len(p) == 0 {
			t.Errorf("empty word segment in thread ID %q", id)
		}
	}
}

func TestGenerateThreadID_UsesWordLists(t *testing.T) {
	id := generateThreadID()
	parts := strings.Split(id, "-")

	inList := func(word string, list []string) bool {
		for _, w := range list {
			if w == word {
				return true
			}
		}
		return false
	}

	if !inList(parts[0], threadDescriptors) {
		t.Errorf("first word %q not in threadDescriptors", parts[0])
	}
	if !inList(parts[1], threadPlants) {
		t.Errorf("second word %q not in threadPlants", parts[1])
	}
	if !inList(parts[2], threadPhenomena) {
		t.Errorf("third word %q not in threadPhenomena", parts[2])
	}
}

func TestGenerateThreadID_Variety(t *testing.T) {
	// Generate 50 IDs — all should be valid, and we should see some variety.
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		id := generateThreadID()
		parts := strings.Split(id, "-")
		if len(parts) != 3 {
			t.Fatalf("invalid ID format: %q", id)
		}
		seen[id] = true
	}
	// With 45k+ combinations, 50 calls should almost never produce duplicates.
	// We allow up to 5 collisions to avoid flakiness.
	if len(seen) < 45 {
		t.Errorf("too many collisions in 50 calls: only %d unique IDs", len(seen))
	}
}

func TestWordListSizes(t *testing.T) {
	if len(threadDescriptors) == 0 {
		t.Error("threadDescriptors is empty")
	}
	if len(threadPlants) == 0 {
		t.Error("threadPlants is empty")
	}
	if len(threadPhenomena) == 0 {
		t.Error("threadPhenomena is empty")
	}
	total := len(threadDescriptors) * len(threadPlants) * len(threadPhenomena)
	if total < 10000 {
		t.Errorf("combination space too small: %d (want ≥ 10000)", total)
	}
}
