package infra

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/imagebuilder/specfile/parser"
)

// Parse parses SpecFile
func Parse(dockerfilePath string) ([]Command, error) {
	file, err := os.Open(dockerfilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	parsed, err := parser.Parse(file)
	if err != nil {
		return nil, err
	}

	commands := make([]Command, 0, len(parsed.AST.Children))
	for _, child := range parsed.AST.Children {
		args := []string{}
		for arg := child.Next; arg != nil; arg = arg.Next {
			args = append(args, arg.Value)
		}

		var cmds []Command
		var err error
		switch strings.ToLower(child.Value) {
		case "from":
			cmds, err = cmdFrom(args)
		case "params":
			cmds, err = cmdParams(args)
		case "run":
			cmds, err = cmdRun(args)
		case "include":
			cmds, err = cmdInclude(args)
		default:
			return nil, fmt.Errorf("unknown command '%s' in line %d", child.Value, child.StartLine)
		}

		if err != nil {
			return nil, fmt.Errorf("error in line %d of %s command: %w", child.StartLine, child.Value, err)
		}

		commands = append(commands, cmds...)
	}
	return commands, nil
}

func cmdFrom(args []string) ([]Command, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("incorrect number of arguments, expected: 1, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}

	buildKey, err := types.ParseBuildKey(args[0])
	if err != nil {
		return nil, err
	}

	return []Command{From(buildKey)}, nil
}

func cmdParams(args []string) ([]Command, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no arguments passed")
	}
	return []Command{Params(args...)}, nil
}

func cmdRun(args []string) ([]Command, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("incorrect number of arguments, expected: 1, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}
	return []Command{Run(args[0])}, nil
}

func cmdInclude(args []string) ([]Command, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no arguments passed")
	}

	res := []Command{}
	for _, arg := range args {
		if arg == "" {
			return nil, errors.New("empty argument passed")
		}

		cmds, err := Parse(arg)
		if err != nil {
			return nil, err
		}
		res = append(res, cmds...)
	}
	return res, nil
}
