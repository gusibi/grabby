package ai

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	domainai "go-server/internal/domain/ai"
)

// Selection is one concrete (profile, model) slot chosen for a request.
//
// The rotation unit is the model, not the provider: a provider configured with
// several free models is used model-by-model, and only then does traffic move
// on to the next provider. A provider with a single model behaves exactly like
// the old profile-level rotation.
type Selection struct {
	Profile domainai.AIProviderProfile
	Model   string
}

// Key identifies the slot for health tracking (profile + model).
func (s Selection) Key() string { return s.Profile.ID + "|" + s.Model }

// unhealthyEntry tracks when a slot can be retried.
type unhealthyEntry struct {
	until time.Time
}

// slot is one (profile, model) pair in priority order.
type slot struct {
	profileID string
	model     string
}

// ProfileSelector manages multi-model selection strategies:
//   - single: prefer the active profile's models, then fall back by priority
//   - round-robin: cycles through every (profile, model) slot
//   - failover: returns the highest-priority healthy slot
//
// Slots are ordered by profile priority and, within a profile, by the order the
// models were configured in. So a provider with [A, B] followed by a provider
// with [C] rotates A → B → C → A …
type ProfileSelector struct {
	mu           sync.Mutex
	strategy     string
	activeID     string
	profiles     map[string]domainai.AIProviderProfile
	slots        []slot
	rrIndex      atomic.Int64
	unhealthy    map[string]unhealthyEntry
	unhealthyTTL time.Duration
}

// NewProfileSelector creates a selector from settings.
// Profiles without any model (and disabled profiles) are skipped, because a
// slot without a model cannot be called.
func NewProfileSelector(settings domainai.AISettings) *ProfileSelector {
	enabled := make([]domainai.AIProviderProfile, 0, len(settings.Profiles))
	for _, p := range settings.Profiles {
		if p.Disabled {
			continue
		}
		models := p.EffectiveModels()
		if len(models) == 0 {
			continue
		}
		p.Models = models
		enabled = append(enabled, p)
	}
	sort.SliceStable(enabled, func(i, j int) bool {
		return enabled[i].Priority < enabled[j].Priority
	})

	slots := make([]slot, 0, len(enabled))
	profiles := make(map[string]domainai.AIProviderProfile, len(enabled))
	for _, p := range enabled {
		profiles[p.ID] = p
		for _, m := range p.Models {
			slots = append(slots, slot{profileID: p.ID, model: m})
		}
	}

	return &ProfileSelector{
		strategy:     settings.Strategy,
		activeID:     settings.ActiveProfileID,
		profiles:     profiles,
		slots:        slots,
		unhealthy:    make(map[string]unhealthyEntry),
		unhealthyTTL: 5 * time.Minute,
	}
}

// EnabledCount returns the number of enabled (profile, model) slots.
func (s *ProfileSelector) EnabledCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.slots)
}

// Next returns the next slot to use based on the strategy.
// Returns nil if no enabled slots are available.
func (s *ProfileSelector) Next() *Selection {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.slots) == 0 {
		return nil
	}

	switch s.strategy {
	case "round-robin":
		return s.nextRoundRobin()
	case "failover":
		return s.nextOrdered()
	default: // "single"
		return s.nextPreferred()
	}
}

// MarkUnhealthy marks a slot as temporarily unavailable.
func (s *ProfileSelector) MarkUnhealthy(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unhealthy[key] = unhealthyEntry{until: time.Now().Add(s.unhealthyTTL)}
}

// MarkHealthy removes a slot from the unhealthy set (on success).
func (s *ProfileSelector) MarkHealthy(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.unhealthy, key)
}

// --- internal helpers (caller must hold mu) ---

func (s *ProfileSelector) selection(sl slot) *Selection {
	p, ok := s.profiles[sl.profileID]
	if !ok {
		return nil
	}
	return &Selection{Profile: p, Model: sl.model}
}

// available reports whether a slot may be used now, clearing expired entries.
func (s *ProfileSelector) available(sl slot) bool {
	key := sl.profileID + "|" + sl.model
	entry, bad := s.unhealthy[key]
	if !bad {
		return true
	}
	if time.Now().After(entry.until) {
		delete(s.unhealthy, key)
		return true
	}
	return false
}

// nextPreferred keeps the active profile first (so "single" still means "use
// what I selected"), but a failing model inside it falls through to its next
// model and then to the remaining profiles by priority.
func (s *ProfileSelector) nextPreferred() *Selection {
	for _, sl := range s.slots {
		if sl.profileID == s.activeID && s.available(sl) {
			return s.selection(sl)
		}
	}
	return s.nextOrdered()
}

func (s *ProfileSelector) nextOrdered() *Selection {
	for _, sl := range s.slots {
		if s.available(sl) {
			return s.selection(sl)
		}
	}
	// Everything is marked unhealthy: clear and retry the first slot so the
	// engine does not deadlock on a transient outage.
	s.unhealthy = make(map[string]unhealthyEntry)
	return s.selection(s.slots[0])
}

func (s *ProfileSelector) nextRoundRobin() *Selection {
	n := len(s.slots)
	start := int(s.rrIndex.Add(1)-1) % n
	for i := 0; i < n; i++ {
		sl := s.slots[(start+i)%n]
		if s.available(sl) {
			return s.selection(sl)
		}
	}
	// All unhealthy — clear and return the slot at the rotation cursor.
	s.unhealthy = make(map[string]unhealthyEntry)
	return s.selection(s.slots[start])
}
