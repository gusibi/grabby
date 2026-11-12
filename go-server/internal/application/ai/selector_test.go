package ai

import (
	"testing"

	domainai "go-server/internal/domain/ai"
)

func makeSettings(strategy string, profiles ...domainai.AIProviderProfile) domainai.AISettings {
	return domainai.AISettings{Enabled: true, Strategy: strategy, Profiles: profiles}
}

func selectionKey(s *Selection) string {
	if s == nil {
		return "<nil>"
	}
	return s.Profile.ID + "/" + s.Model
}

func collect(sel *ProfileSelector, n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, selectionKey(sel.Next()))
	}
	return out
}

func assertSequence(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("sequence = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sequence = %v, want %v", got, want)
		}
	}
}

// The core requirement: a provider with several models is used model-by-model
// before moving on to the next provider.
func TestRoundRobinRotatesModelsWithinProvider(t *testing.T) {
	sel := NewProfileSelector(makeSettings("round-robin",
		domainai.AIProviderProfile{ID: "a", Priority: 1, Models: []string{"a1", "a2"}},
		domainai.AIProviderProfile{ID: "b", Priority: 2, Models: []string{"b1"}},
	))

	assertSequence(t, collect(sel, 5), []string{
		"a/a1", "a/a2", "b/b1", "a/a1", "a/a2",
	})
}

func TestRoundRobinCountsModelsNotProfiles(t *testing.T) {
	sel := NewProfileSelector(makeSettings("round-robin",
		domainai.AIProviderProfile{ID: "a", Priority: 1, Models: []string{"a1", "a2", "a3"}},
		domainai.AIProviderProfile{ID: "b", Priority: 2, Models: []string{"b1"}},
	))
	if got := sel.EnabledCount(); got != 4 {
		t.Fatalf("EnabledCount() = %d, want 4", got)
	}
}

func TestLegacySingleModelProfileStillWorks(t *testing.T) {
	sel := NewProfileSelector(makeSettings("round-robin",
		domainai.AIProviderProfile{ID: "old", Priority: 1, Model: "gpt-4o"},
	))
	assertSequence(t, collect(sel, 2), []string{"old/gpt-4o", "old/gpt-4o"})
}

func TestProfilesWithoutModelsAreSkipped(t *testing.T) {
	sel := NewProfileSelector(makeSettings("round-robin",
		domainai.AIProviderProfile{ID: "empty", Priority: 1},
		domainai.AIProviderProfile{ID: "disabled", Priority: 2, Disabled: true, Models: []string{"d1"}},
		domainai.AIProviderProfile{ID: "ok", Priority: 3, Models: []string{"m1"}},
	))
	if got := sel.EnabledCount(); got != 1 {
		t.Fatalf("EnabledCount() = %d, want 1", got)
	}
	assertSequence(t, collect(sel, 1), []string{"ok/m1"})
}

func TestFailoverSkipsUnhealthyModelButKeepsProvider(t *testing.T) {
	sel := NewProfileSelector(makeSettings("failover",
		domainai.AIProviderProfile{ID: "a", Priority: 1, Models: []string{"a1", "a2"}},
		domainai.AIProviderProfile{ID: "b", Priority: 2, Models: []string{"b1"}},
	))

	first := sel.Next()
	assertSequence(t, []string{selectionKey(first)}, []string{"a/a1"})
	sel.MarkUnhealthy(first.Key())

	// a1 is unhealthy for 5 minutes; the next model of the SAME provider wins.
	second := sel.Next()
	assertSequence(t, []string{selectionKey(second)}, []string{"a/a2"})

	sel.MarkUnhealthy(second.Key())
	assertSequence(t, []string{selectionKey(sel.Next())}, []string{"b/b1"})
}

func TestSinglePrefersActiveProfileThenFallsBack(t *testing.T) {
	sel := NewProfileSelector(domainai.AISettings{
		Enabled:         true,
		Strategy:        "single",
		ActiveProfileID: "b",
		Profiles: []domainai.AIProviderProfile{
			{ID: "a", Priority: 1, Models: []string{"a1"}},
			{ID: "b", Priority: 2, Models: []string{"b1", "b2"}},
		},
	})

	assertSequence(t, []string{selectionKey(sel.Next())}, []string{"b/b1"})

	sel.MarkUnhealthy("b|b1")
	assertSequence(t, []string{selectionKey(sel.Next())}, []string{"b/b2"})

	sel.MarkUnhealthy("b|b2")
	assertSequence(t, []string{selectionKey(sel.Next())}, []string{"a/a1"})
}

func TestMarkHealthyRestoresSlot(t *testing.T) {
	sel := NewProfileSelector(makeSettings("failover",
		domainai.AIProviderProfile{ID: "a", Priority: 1, Models: []string{"a1"}},
		domainai.AIProviderProfile{ID: "b", Priority: 2, Models: []string{"b1"}},
	))

	sel.MarkUnhealthy("a|a1")
	assertSequence(t, []string{selectionKey(sel.Next())}, []string{"b/b1"})

	sel.MarkHealthy("a|a1")
	assertSequence(t, []string{selectionKey(sel.Next())}, []string{"a/a1"})
}
