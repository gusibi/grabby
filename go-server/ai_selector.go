package main

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// unhealthyEntry tracks when a profile can be retried.
type unhealthyEntry struct {
	until time.Time
}

// ProfileSelector manages multi-profile selection strategies:
//   - single: always returns the active profile
//   - round-robin: cycles through enabled profiles
//   - failover: returns highest-priority healthy profile; falls back on error
type ProfileSelector struct {
	mu          sync.Mutex
	strategy    string
	activeID    string
	profiles    []AIProviderProfile // enabled, sorted by priority
	rrIndex     atomic.Int64
	unhealthy   map[string]unhealthyEntry
	unhealthyTTL time.Duration
}

// NewProfileSelector creates a selector from settings.
// Only non-disabled profiles are included.
func NewProfileSelector(settings AISettings) *ProfileSelector {
	enabled := make([]AIProviderProfile, 0)
	for _, p := range settings.Profiles {
		if !p.Disabled {
			enabled = append(enabled, p)
		}
	}
	sort.Slice(enabled, func(i, j int) bool {
		return enabled[i].Priority < enabled[j].Priority
	})

	return &ProfileSelector{
		strategy:     settings.Strategy,
		activeID:     settings.ActiveProfileID,
		profiles:     enabled,
		unhealthy:    make(map[string]unhealthyEntry),
		unhealthyTTL: 5 * time.Minute,
	}
}

// EnabledCount returns the number of enabled profiles.
func (s *ProfileSelector) EnabledCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.profiles)
}

// Next returns the next profile to use based on the strategy.
// Returns nil if no enabled profiles are available.
func (s *ProfileSelector) Next() *AIProviderProfile {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.profiles) == 0 {
		return nil
	}

	switch s.strategy {
	case "round-robin":
		return s.nextRoundRobin()
	case "failover":
		return s.nextFailover()
	default: // "single"
		return s.findActive()
	}
}

// MarkUnhealthy marks a profile as temporarily unavailable.
func (s *ProfileSelector) MarkUnhealthy(profileID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unhealthy[profileID] = unhealthyEntry{until: time.Now().Add(s.unhealthyTTL)}
}

// MarkHealthy removes a profile from the unhealthy set (on success).
func (s *ProfileSelector) MarkHealthy(profileID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.unhealthy, profileID)
}

// --- internal helpers (caller must hold mu) ---

func (s *ProfileSelector) findActive() *AIProviderProfile {
	for i := range s.profiles {
		if s.profiles[i].ID == s.activeID {
			return &s.profiles[i]
		}
	}
	// Active profile not found in enabled list; fall back to first
	if len(s.profiles) > 0 {
		return &s.profiles[0]
	}
	return nil
}

func (s *ProfileSelector) nextRoundRobin() *AIProviderProfile {
	n := len(s.profiles)
	if n == 0 {
		return nil
	}
	// Try all profiles starting from rrIndex, skip unhealthy
	start := int(s.rrIndex.Add(1)-1) % n
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		p := &s.profiles[idx]
		if _, bad := s.unhealthy[p.ID]; !bad {
			return p
		}
	}
	// All unhealthy — clear and return first
	s.unhealthy = make(map[string]unhealthyEntry)
	return &s.profiles[0]
}

func (s *ProfileSelector) nextFailover() *AIProviderProfile {
	// profiles already sorted by priority
	for i := range s.profiles {
		p := &s.profiles[i]
		entry, bad := s.unhealthy[p.ID]
		if !bad || time.Now().After(entry.until) {
			// healthy or TTL expired — use it
			delete(s.unhealthy, p.ID)
			return p
		}
	}
	// All unhealthy, reset and try first
	s.unhealthy = make(map[string]unhealthyEntry)
	return &s.profiles[0]
}
