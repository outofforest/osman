package types

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ErrImageDoesNotExist is returned if source image does not exist
var ErrImageDoesNotExist = errors.New("image does not exist")

const alphabet = "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const buildIDPrefixLength = 3
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

// RandomString generates random string of fixed length
func RandomString(length int) string {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	encode(buf)
	return string(buf)
}

// NewBuildID returns new random build ID
func NewBuildID(buildType BuildType) BuildID {
	if !buildType.IsValid() {
		panic(fmt.Errorf("invalid build type: %s", buildType))
	}
	buildIDCore := string(buildType) + RandomString(buildIDLength)
	return BuildID(buildIDCore + checksum(buildIDCore))
}

// ParseBuildID parses string into build ID and returns error if string is not a valid one
func ParseBuildID(strBuildID string) (BuildID, error) {
	buildID := BuildID(strBuildID)
	if !buildID.IsValid() {
		return "", fmt.Errorf("invalid build ID: '%s'", strBuildID)
	}
	return buildID, nil
}

// BuildType is the type of build
type BuildType string

// IsValid verifies if build type is valid
func (bt BuildType) IsValid() bool {
	_, exists := buildTypes[bt]
	return exists
}

// Properties returns properties of build type
func (bt BuildType) Properties() BuildTypeProperties {
	return buildTypes[bt]
}

// BuildTypeProperties contains properties of build type
type BuildTypeProperties struct {
	// Cloneable means image may be cloned
	Cloneable bool

	// Mountable means image stays mounted
	Mountable bool

	// VM means vm in libvirt is defined for this image
	VM bool
}

const (
	// BuildTypeImage is the image build type
	BuildTypeImage BuildType = "iid"

	// BuildTypeMount is the mount build type
	BuildTypeMount BuildType = "mid"

	// BuildTypeVM is the vm build type
	BuildTypeVM BuildType = "vid"
)

var buildTypes = map[BuildType]BuildTypeProperties{
	BuildTypeImage: {
		Cloneable: true,
	},
	BuildTypeMount: {
		Mountable: true,
	},
	BuildTypeVM: {
		Mountable: true,
		VM:        true,
	},
}

// BuildID is unique ID of build
type BuildID string

// Type returns type of build encoded inside build ID
func (bid BuildID) Type() BuildType {
	return BuildType(bid[:buildIDPrefixLength])
}

// IsValid verifies if format of build ID is valid
func (bid BuildID) IsValid() bool {
	dataLen := buildIDPrefixLength + buildIDLength
	if len(bid) != dataLen+checksumLength {
		return false
	}
	if !bid.Type().IsValid() {
		return false
	}
	return checksum(string(bid[:dataLen])) == string(bid[dataLen:])
}

// IsValidType verifies if format of build ID is valid and type matches
func (bid BuildID) IsValidType(buildType BuildType) bool {
	if !bid.IsValid() {
		return false
	}
	return bid.Type() == buildType
}

var validRegExp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-_]*$`)

// Tag is the tag of build
type Tag string

// IsValid returns true if tag is valid
func (t Tag) IsValid() bool {
	return validRegExp.MatchString(string(t))
}

// Tags is a sortable representation of slice of tags
type Tags []Tag

func (t Tags) String() string {
	values := make([]string, 0, len(t))
	for _, tag := range t {
		values = append(values, string(tag))
	}
	sort.Strings(values)
	return strings.Join(values, ", ")
}

// IsNameValid returns true if name is valid
func IsNameValid(name string) bool {
	for t := range buildTypes {
		if strings.HasPrefix(name, string(t)) {
			return false
		}
	}
	return validRegExp.MatchString(name)
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

// IsValid returns true if build key is valid
func (bk BuildKey) IsValid() bool {
	return IsNameValid(bk.Name) && bk.Tag.IsValid()
}

// Params is a list of params configured on image
type Params []string

func (p Params) String() string {
	values := make([]string, len(p))
	copy(values, p)
	sort.Strings(values)

	return strings.Join(values, ", ")
}

// ImageManifest contains info about built image
type ImageManifest struct {
	BuildID BuildID
	BasedOn BuildID
	Params  Params
}

// BuildInfo stores all the information about build
type BuildInfo struct {
	BuildID   BuildID
	BasedOn   BuildID
	CreatedAt time.Time
	Name      string
	Tags      Tags
	Params    Params
	Mounted   string
}
