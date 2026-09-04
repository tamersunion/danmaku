package api

import (
	"sync"

	"github.com/philippseith/signalr"
)

type liveHub struct {
	signalr.Hub
	registry *groupRegistry
}

func (h *liveHub) Connection(group string) {
	connectionID := h.ConnectionID()
	h.Groups().AddToGroup(group, connectionID)
	h.registry.add(group, connectionID)
}

func (h *liveHub) SendMessage(group, user, message string) {
	caller := h.ConnectionID()
	for _, connectionID := range h.registry.members(group) {
		if connectionID != caller {
			h.Clients().Client(connectionID).Send("ReceiveMessage", user, message)
		}
	}
}

func (h *liveHub) OnDisconnected(connectionID string) {
	h.registry.remove(connectionID)
}

type groupRegistry struct {
	mu      sync.RWMutex
	groups  map[string]map[string]struct{}
	reverse map[string]map[string]struct{}
}

func newGroupRegistry() *groupRegistry {
	return &groupRegistry{groups: make(map[string]map[string]struct{}), reverse: make(map[string]map[string]struct{})}
}

func (r *groupRegistry) add(group, connectionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.groups[group] == nil {
		r.groups[group] = make(map[string]struct{})
	}
	if r.reverse[connectionID] == nil {
		r.reverse[connectionID] = make(map[string]struct{})
	}
	r.groups[group][connectionID] = struct{}{}
	r.reverse[connectionID][group] = struct{}{}
}

func (r *groupRegistry) members(group string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(r.groups[group]))
	for connectionID := range r.groups[group] {
		result = append(result, connectionID)
	}
	return result
}

func (r *groupRegistry) remove(connectionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for group := range r.reverse[connectionID] {
		delete(r.groups[group], connectionID)
		if len(r.groups[group]) == 0 {
			delete(r.groups, group)
		}
	}
	delete(r.reverse, connectionID)
}
