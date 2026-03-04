package views

import "sync"

// Registry holds all registered view definitions and provides an inverted index by table name.
type Registry struct {
	mu      sync.RWMutex
	views   map[string]*ViewDef
	byTable map[string][]*ViewDef
}

// NewRegistry creates an empty view registry.
func NewRegistry() *Registry {
	return &Registry{
		views:   make(map[string]*ViewDef),
		byTable: make(map[string][]*ViewDef),
	}
}

// Register adds a view definition and updates the inverted index.
func (r *Registry) Register(def *ViewDef) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.views[def.Key] = def
	for _, td := range def.Tables {
		r.byTable[td.Table] = append(r.byTable[td.Table], def)
	}
}

// Get returns a view definition by key.
func (r *Registry) Get(key string) (*ViewDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.views[key]
	return def, ok
}

// GetByTable returns all view definitions that depend on the given table.
func (r *Registry) GetByTable(table string) []*ViewDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byTable[table]
}

// All returns all registered view definitions.
func (r *Registry) All() []*ViewDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ViewDef, 0, len(r.views))
	for _, v := range r.views {
		out = append(out, v)
	}
	return out
}
