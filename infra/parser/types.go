package parser

import "github.com/wojciech-malota-wojcik/imagebuilder/infra/description"

// Parser parses image description from file
type Parser interface {
	// Parse parses file and converts it to commands
	Parse(filePath string) ([]description.Command, error)
}
