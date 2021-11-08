package infra

// NewRepository creates new image repository
func NewRepository() *Repository {
	return &Repository{
		images: map[string]*Descriptor{},
	}
}

// Repository is an image repository
type Repository struct {
	images map[string]*Descriptor
}

// Store stores image descriptor in repository
func (r *Repository) Store(img *Descriptor) {
	r.images[img.Name()] = img
}

// Retrieve retrieves image descriptor from repository
func (r *Repository) Retrieve(name string) *Descriptor {
	return r.images[name]
}
