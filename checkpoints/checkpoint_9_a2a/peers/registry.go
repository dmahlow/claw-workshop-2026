package peers

import (
	"strings"
	"sync"

	"myclaw/a2a"
)

// Registry stores discovered peer agents in a thread-safe map.
type Registry struct {
	mu    sync.RWMutex
	peers map[string]*a2a.AgentCard // URL -> AgentCard
}

// NewRegistry creates a new empty peer registry.
func NewRegistry() *Registry {
	return &Registry{
		peers: make(map[string]*a2a.AgentCard),
	}
}

// Add stores a peer's Agent Card in the registry, keyed by URL.
func (r *Registry) Add(card *a2a.AgentCard) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[card.URL] = card
}

// Get returns a peer's Agent Card by URL.
func (r *Registry) Get(url string) (*a2a.AgentCard, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	card, ok := r.peers[url]
	return card, ok
}

// All returns a copy of all known peers.
func (r *Registry) All() map[string]*a2a.AgentCard {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]*a2a.AgentCard, len(r.peers))
	for k, v := range r.peers {
		result[k] = v
	}
	return result
}

// FindBySkill searches the registry for peers that have a skill matching the query.
// The search is case-insensitive and checks skill IDs, names, descriptions, and tags.
func (r *Registry) FindBySkill(query string) []*a2a.AgentCard {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	var matches []*a2a.AgentCard

	for _, card := range r.peers {
		if peerMatchesSkill(card, query) {
			matches = append(matches, card)
		}
	}
	return matches
}

func peerMatchesSkill(card *a2a.AgentCard, query string) bool {
	for _, skill := range card.Skills {
		if strings.Contains(strings.ToLower(skill.ID), query) ||
			strings.Contains(strings.ToLower(skill.Name), query) ||
			strings.Contains(strings.ToLower(skill.Description), query) {
			return true
		}
		for _, tag := range skill.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				return true
			}
		}
	}
	return false
}
