package format

import (
	"encoding/json"

	"github.com/ridge/must"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
)

// NewJSONFormatter returns formatter converting build list into json string
func NewJSONFormatter() Formatter {
	return &jsonFormatter{}
}

type jsonFormatter struct {
}

// Format formats build list into json string
func (f *jsonFormatter) Format(builds []storage.BuildInfo) string {
	return string(must.Bytes(json.MarshalIndent(builds, "", "  ")))
}
