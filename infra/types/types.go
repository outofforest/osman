package types

// BuildID is unique ID of build
type BuildID string

// Tag is the tag of build
type Tag string

// TagSlice is a sortable representation of slice of tags
type TagSlice []Tag

func (x TagSlice) Len() int           { return len(x) }
func (x TagSlice) Less(i, j int) bool { return x[i] < x[j] }
func (x TagSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
