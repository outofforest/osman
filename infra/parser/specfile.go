package parser

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/outofforest/osman/infra/description"
	"github.com/outofforest/osman/infra/types"
	"github.com/outofforest/osman/specfile/parser"
)

// NewSpecFileParser creates new specfile parser.
func NewSpecFileParser() Parser {
	return &specFileParser{}
}

// Parser parses image description from file.
type specFileParser struct {
}

// Parse parses commands from specfile.
func (p *specFileParser) Parse(filePath string) ([]description.Command, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer file.Close()

	parsed, err := parser.Parse(file)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	commands := make([]description.Command, 0, len(parsed.AST.Children))
	for _, child := range parsed.AST.Children {
		args := []string{}
		for arg := child.Next; arg != nil; arg = arg.Next {
			args = append(args, arg.Value)
		}

		var cmds []description.Command
		var err error
		switch strings.ToLower(child.Value) {
		case "from":
			cmds, err = p.cmdFrom(args)
		case "params":
			cmds, err = p.cmdParams(args)
		case "run":
			cmds, err = p.cmdRun(args)
		case "include":
			cmds, err = p.cmdInclude(args)
		case "boot":
			cmds, err = p.cmdBoot(args)
		default:
			return nil, errors.Errorf("unknown command '%s' in line %d", child.Value, child.StartLine)
		}

		if err != nil {
			return nil, errors.WithStack(fmt.Errorf("error in line %d of %s command: %w", child.StartLine, child.Value, err))
		}

		commands = append(commands, cmds...)
	}
	return commands, nil
}

func (p *specFileParser) cmdFrom(args []string) ([]description.Command, error) {
	if len(args) != 1 {
		return nil, errors.Errorf("incorrect number of arguments, expected: 1, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}

	buildKey, err := types.ParseBuildKey(args[0])
	if err != nil {
		return nil, err
	}

	return []description.Command{description.From(buildKey)}, nil
}

func (p *specFileParser) cmdParams(args []string) ([]description.Command, error) {
	if len(args) == 0 {
		return nil, errors.Errorf("no arguments passed")
	}
	return []description.Command{description.Params(args...)}, nil
}

func (p *specFileParser) cmdRun(args []string) ([]description.Command, error) {
	if len(args) != 1 {
		return nil, errors.Errorf("incorrect number of arguments, expected: 1, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}
	return []description.Command{description.Run(args[0])}, nil
}

func (p *specFileParser) cmdInclude(args []string) ([]description.Command, error) {
	if len(args) == 0 {
		return nil, errors.New("no arguments passed")
	}

	res := []description.Command{}
	for _, arg := range args {
		if arg == "" {
			return nil, errors.New("empty argument passed")
		}

		cmds, err := p.Parse(arg)
		if err != nil {
			return nil, err
		}
		res = append(res, cmds...)
	}
	return res, nil
}

func (p *specFileParser) cmdBoot(args []string) ([]description.Command, error) {
	if len(args) == 0 {
		return nil, errors.New("no arguments passed")
	}

	if args[0] == "" {
		return nil, errors.New("empty argument passed")
	}

	var params []string
	if len(args) > 1 {
		params = make([]string, 0, len(args)-1)

		for _, arg := range args[1:] {
			if arg == "" {
				return nil, errors.New("empty argument passed")
			}
			params = append(params, arg)
		}
	}
	return []description.Command{description.Boot(args[0], params)}, nil
}
