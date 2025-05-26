package rpc

// Registry holds JSON-RPC method handlers
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry creates a new registry
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register adds a handler for a method name
func (r *Registry) Register(name string, h Handler) {
	r.handlers[name] = h
}

// Get returns the handler for a method name
func (r *Registry) Get(name string) Handler {
	return r.handlers[name]
}
