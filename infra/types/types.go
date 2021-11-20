package types

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"regexp"
	"strings"
)

const alphabet = "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const buildIDPrefix = "bid"
const buildIDLength = 16
const checksumLength = 4

var crcTable *crc32.Table

func init() {
	crcTable = crc32.MakeTable(crc32.Castagnoli)
}

func encode(buf []byte) {
	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
}

func checksum(data string) string {
	buf := make([]byte, checksumLength)
	binary.LittleEndian.PutUint32(buf, crc32.Checksum([]byte(data), crcTable))
	encode(buf)
	return string(buf)
}

// NewBuildID returns new random build ID
func NewBuildID() BuildID {
	buf := make([]byte, buildIDLength)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	encode(buf)
	return BuildID(buildIDPrefix + string(buf) + checksum(string(buf)))
}

// ParseBuildID parses string into build ID and returns error if string is not a valid one
func ParseBuildID(strBuildID string) (BuildID, error) {
	buildID := BuildID(strBuildID)
	if !buildID.IsValid() {
		return "", fmt.Errorf("invalid build ID: '%s'", strBuildID)
	}
	return buildID, nil
}

// BuildID is unique ID of build
type BuildID string

// IsValid verifies if format of build ID is valid
func (bid BuildID) IsValid() bool {
	if len(bid) != len(buildIDPrefix)+buildIDLength+checksumLength {
		return false
	}
	if !strings.HasPrefix(string(bid), buildIDPrefix) {
		return false
	}
	return checksum(string(bid[len(buildIDPrefix):len(buildIDPrefix)+buildIDLength])) == string(bid[len(bid)-checksumLength:])
}

// Tag is the tag of build
type Tag string

// IsValid returns true if tag is valid
func (t Tag) IsValid() bool {
	return regExp.MatchString(string(t))
}

// Tags is a sortable representation of slice of tags
type Tags []Tag

func (x Tags) String() string {
	strs := make([]string, 0, len(x))
	for _, tag := range x {
		strs = append(strs, string(tag))
	}
	return strings.Join(strs, ", ")
}
func (x Tags) Len() int           { return len(x) }
func (x Tags) Less(i, j int) bool { return x[i] < x[j] }
func (x Tags) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

var regExp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-_]*$`)

// IsNameValid returns true if name is valid
func IsNameValid(name string) bool {
	if strings.HasPrefix(name, buildIDPrefix) {
		return false
	}
	return regExp.MatchString(name)
}

// NewBuildKey returns new build key
func NewBuildKey(name string, tag Tag) BuildKey {
	return BuildKey{Name: name, Tag: tag}
}

// ParseBuildKey parses string into build key and returns error if string is not a valid one
func ParseBuildKey(strBuildKey string) (BuildKey, error) {
	if strBuildKey == "" {
		return BuildKey{}, errors.New("empty build key received")
	}
	parts := strings.SplitN(strBuildKey, ":", 2)
	name := parts[0]
	if name != "" && !IsNameValid(name) {
		return BuildKey{}, fmt.Errorf("name '%s' is invalid", name)
	}

	var tag Tag
	if len(parts) == 2 {
		tag = Tag(parts[1])
		if tag == "" {
			return BuildKey{}, errors.New("empty tag received")
		}
	}
	if tag != "" && !tag.IsValid() {
		return BuildKey{}, fmt.Errorf("tag '%s' is invalid", tag)
	}

	return BuildKey{Name: name, Tag: tag}, nil
}

// BuildKey represents Name-Tag pair
type BuildKey struct {
	Name string
	Tag  Tag
}

// String returns string representation of build key
func (bk BuildKey) String() string {
	return fmt.Sprintf("%s:%s", bk.Name, bk.Tag)
}
