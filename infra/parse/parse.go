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

		var cmd infra.Command
		var err error
		switch strings.ToLower(child.Value) {
		case "from":
			cmd, err = cmdFrom(args)
		case "label":
			cmd, err = cmdLabel(args)
		case "copy":
			cmd, err = cmdCopy(args)
		case "run":
			cmd, err = cmdRun(args)
		default:
			return nil, fmt.Errorf("unknown command '%s' in line %d", child.Value, child.StartLine)
		}

		if err != nil {
			return nil, fmt.Errorf("error in line %d of %s command: %w", child.StartLine, child.Value, err)
		}

		commands = append(commands, cmd)
	}
	return commands, nil
}

func cmdFrom(args []string) (infra.Command, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("incorrect number of arguments, expected: 1, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}
	return infra.From(args[0]), nil
}

func cmdLabel(args []string) (infra.Command, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("incorrect number of arguments, expected: 2, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}
	if args[1] == "" {
		return nil, errors.New("second argument is empty")
	}
	return infra.Label(args[0], args[1]), nil
}

func cmdCopy(args []string) (infra.Command, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("incorrect number of arguments, expected: 2, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}
	if args[1] == "" {
		return nil, errors.New("second argument is empty")
	}
	return infra.Copy(args[0], args[1]), nil
}

func cmdRun(args []string) (infra.Command, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("incorrect number of arguments, expected: 1, got: %d", len(args))
	}
	if args[0] == "" {
		return nil, errors.New("first argument is empty")
	}
	return infra.Run(args[0]), nil
}
