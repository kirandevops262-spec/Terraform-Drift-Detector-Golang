package cloud

import (
	"context"

	"github.com/terraform-drift-detector/golang/internal/extract"
)

// FetchScope defines what to fetch from a cloud provider.
type FetchScope struct {
	Regions       []string
	ResourceTypes []string
	Tags          map[string]string
}

// Provider fetches live cloud resources.
type Provider interface {
	Name() string
	Fetch(ctx context.Context, scope FetchScope) ([]extract.RawCloudResource, error)
	SupportedTypes() []string
}

// Registry holds registered cloud providers.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}
