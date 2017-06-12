package servenv

import (
	"sync"
)

// Hooks holds a list of parameter-less functions to call whenever the set is
// triggered with Fire().
type Hooks struct {
	funcs []func()
	mu    sync.Mutex
}

// Add appends the given function to the list to be triggered.
func (h *Hooks) Add(f func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.funcs = append(h.funcs, f)
}

// Fire calls all the functions in a given Hooks list.
func (h *Hooks) Fire() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, f := range h.funcs {
		f()
	}
}
