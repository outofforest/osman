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
func (f *jsonFormatter) Format(slice interface{}, fieldsToPrint ...string) string {
	marshaled := must.Bytes(json.MarshalIndent(slice, "", "  "))
	if fieldsToPrint == nil {
		return string(marshaled)
	}

	var list []map[string]interface{}
	must.OK(json.Unmarshal(marshaled, &list))
	enabledFields := mapEnabledFields(fieldsToPrint)
	for _, m := range list {
		for k := range m {
			if !enabledFields[k] {
				delete(m, k)
			}
		}
	}

	return string(must.Bytes(json.MarshalIndent(list, "", "  ")))
}
