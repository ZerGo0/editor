package ghtest

import "sync"

type Calls struct {
	mu    sync.Mutex
	names []string
}

func (c *Calls) Add(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.names = append(c.names, name)
}

func (c *Calls) Count(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, got := range c.names {
		if got == name {
			n++
		}
	}
	return n
}

func (c *Calls) Names() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.names...)
}
