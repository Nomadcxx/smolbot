package theme

import "sync"

var (
	mu       sync.RWMutex
	current  *Theme
	registry = map[string]*Theme{}
)

func Register(t *Theme) {
	mu.Lock()
	defer mu.Unlock()
	registry[t.Name] = t
}

func Set(name string) bool {
	mu.Lock()
	defer mu.Unlock()
	t, ok := registry[name]
	if !ok {
		return false
	}
	current = t
	return true
}

func Current() *Theme {
	mu.RLock()
	if current != nil {
		defer mu.RUnlock()
		return current
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()
	if current != nil {
		return current
	}
	for _, t := range registry {
		current = t
		break
	}
	return current
}

func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
