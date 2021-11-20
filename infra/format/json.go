package format

import (
	"encoding/json"

	"github.com/ridge/must"
)

// NewJSONFormatter returns formatter converting slice into json string
func NewJSONFormatter() Formatter {
	return &jsonFormatter{}
}

type jsonFormatter struct {
}

// Format formats slice into json string
func (f *jsonFormatter) Format(slice interface{}) string {
	return string(must.Bytes(json.MarshalIndent(slice, "", "  ")))
}
