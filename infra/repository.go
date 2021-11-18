package infra

import "github.com/wojciech-malota-wojcik/imagebuilder/infra/types"

type repoKey struct {
	name string
	tag  types.Tag
}

// NewRepository creates new image repository
func NewRepository() *Repository {
	return &Repository{
		images: map[repoKey]*Descriptor{},
	}
}

// Repository is an image repository
type Repository struct {
	images map[repoKey]*Descriptor
}

// Store stores image descriptor in repository
func (r *Repository) Store(img *Descriptor) {
	for _, tag := range img.Tags() {
		r.images[repoKey{name: img.Name(), tag: tag}] = img
	}
}

// Retrieve retrieves image descriptor from repository
func (r *Repository) Retrieve(name string, tag types.Tag) *Descriptor {
	return r.images[repoKey{name: name, tag: tag}]
}
