package types

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strings"
)

const alphabet = "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const buildIDLength = 16
const checksumLength = 4

// BuildIDPrefix is the prefix reserved for build IDs
const BuildIDPrefix = "bid"

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
	return BuildID(BuildIDPrefix + string(buf) + checksum(string(buf)))
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
	if len(bid) != len(BuildIDPrefix)+buildIDLength+checksumLength {
		return false
	}
	if !strings.HasPrefix(string(bid), BuildIDPrefix) {
		return false
	}
	return checksum(string(bid[len(BuildIDPrefix):len(BuildIDPrefix)+buildIDLength])) == string(bid[len(bid)-checksumLength:])
}

// Tag is the tag of build
type Tag string

// TagSlice is a sortable representation of slice of tags
type TagSlice []Tag

func (x TagSlice) Len() int           { return len(x) }
func (x TagSlice) Less(i, j int) bool { return x[i] < x[j] }
func (x TagSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
