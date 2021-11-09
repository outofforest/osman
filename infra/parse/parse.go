package parse

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/wojciech-malota-wojcik/imagebuilder/dockerfile/parser"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra"
)

// Parse parses Dockerfile
func Parse(dockerfilePath string) ([]infra.Command, error) {
	file, err := os.Open(dockerfilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	parsed, err := parser.Parse(file)
	if err != nil {
		return nil, err
	}

	commands := make([]infra.Command, 0, len(parsed.AST.Children))
	for _, child := range parsed.AST.Children {
		args := []string{}
		for arg := child.Next; arg != nil; arg = arg.Next {
			args = append(args, arg.Value)
		}

		var cmds []infra.Command
		var err error
		switch strings.ToLower(child.Value) {
		case "from":
			cmds, err = cmdFrom(args)
		case "label":
			cmds, err = cmdLabel(args)
		case "copy":
			cmds, err = cmdCopy(args)
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

func cmdFrom(args []string) ([]infra.Command, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("incorrect number of arguments, expected: 1, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}
	return []infra.Command{infra.From(args[0])}, nil
}

func cmdLabel(args []string) ([]infra.Command, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("incorrect number of arguments, expected: 2, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}
	if args[1] == "" {
		return nil, errors.New("second argument is empty")
	}
	return []infra.Command{infra.Label(args[0], args[1])}, nil
}

func cmdCopy(args []string) ([]infra.Command, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("incorrect number of arguments, expected: 2, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}
	if args[1] == "" {
		return nil, errors.New("second argument is empty")
	}
	return []infra.Command{infra.Copy(args[0], args[1])}, nil
}

func cmdRun(args []string) ([]infra.Command, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("incorrect number of arguments, expected: 1, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}
	return []infra.Command{infra.Run(args[0])}, nil
}

func cmdInclude(args []string) ([]infra.Command, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no arguments passed")
	}

	res := []infra.Command{}
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
