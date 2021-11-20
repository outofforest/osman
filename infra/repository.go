package infra

import (
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// NewRepository creates new image repository
func NewRepository() *Repository {
	return &Repository{
		images: map[types.BuildKey]*Descriptor{},
	}
}

// Repository is an image repository
type Repository struct {
	images map[types.BuildKey]*Descriptor
}

// Store stores image descriptor in repository
func (r *Repository) Store(img *Descriptor) {
	for _, tag := range img.Tags() {
		r.images[types.NewBuildKey(img.Name(), tag)] = img
	}
}

// Retrieve retrieves image descriptor from repository
func (r *Repository) Retrieve(buildKey types.BuildKey) *Descriptor {
	return r.images[buildKey]
}
