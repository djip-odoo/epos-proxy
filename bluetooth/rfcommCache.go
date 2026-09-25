package bluetooth

import "sync"

type rfcommCache struct {
	mu      sync.RWMutex
	entries map[string]*rfcommBinding
}

func (c *rfcommCache) get(address string) (*rfcommBinding, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	b, ok := c.entries[address]
	if !ok || b == nil {
		return nil, false
	}
	copy := *b
	return &copy, true
}

func (c *rfcommCache) set(address string, b *rfcommBinding) {
	if b == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	copy := *b
	c.entries[address] = &copy
}

func (c *rfcommCache) deleteIf(address string, predicate func(*rfcommBinding) bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if b, ok := c.entries[address]; ok {
		if predicate == nil || predicate(b) {
			delete(c.entries, address)
		}
	}
}
