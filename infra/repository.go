package infra

import (
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// NewRepository creates new image repository
func NewRepository() *Repository {
	return &Repository{
		images: map[buildKey]*Descriptor{},
	}
}

// Repository is an image repository
type Repository struct {
	images map[buildKey]*Descriptor
}

// Store stores image descriptor in repository
func (r *Repository) Store(img *Descriptor) {
	for _, tag := range img.Tags() {
		r.images[buildKey{name: img.Name(), tag: tag}] = img
	}
}

// Retrieve retrieves image descriptor from repository
func (r *Repository) Retrieve(name string, tag types.Tag) *Descriptor {
	return r.images[buildKey{name: name, tag: tag}]
}
