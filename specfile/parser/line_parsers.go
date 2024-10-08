package parser

// line parsers are dispatch calls that parse a single unit of text into a
// Node object which contains the whole statement. Dockerfiles have varied
// (but not usually unique, see ONBUILD for a unique example) parsing rules
// per-command, and these unify the processing in a way that makes it
// manageable.

import (
	"encoding/json"
	"strings"
	"unicode"

	"github.com/pkg/errors"
)

var (
	errDockerfileNotStringArray = errors.New("when using JSON array syntax, arrays must be comprised of strings only")
)

// ignore the current argument. This will still leave a command parsed, but
// will not incorporate the arguments into the ast.
func parseIgnore(rest string, d *directives) (*Node, map[string]bool, error) {
	return &Node{}, nil, nil
}

// parses a whitespace-delimited set of arguments. The result is effectively a
// linked list of string arguments.
func parseStringsWhitespaceDelimited(rest string, _ *directives) (*Node, map[string]bool, error) {
	if rest == "" {
		return nil, nil, nil
	}

	node := &Node{}
	rootnode := node
	prevnode := node
	for _, str := range reWhitespace.Split(rest, -1) { // use regexp
		prevnode = node
		node.Value = str
		node.Next = &Node{}
		node = node.Next
	}

	// XXX to get around regexp.Split *always* providing an empty string at the
	// end due to how our loop is constructed, nil out the last node in the
	// chain.
	prevnode.Next = nil

	return rootnode, nil, nil
}

// parseMaybeJSONToList determines if the argument appears to be a JSON array. If
// so, passes to parseJSON; if not, attempts to parse it as a whitespace
// delimited string.
func parseMaybeJSONToList(rest string, d *directives) (*Node, map[string]bool, error) {
	node, attrs, err := parseJSON(rest, d)

	if err == nil {
		return node, attrs, nil
	}
	if errors.Is(err, errDockerfileNotStringArray) {
		return nil, nil, err
	}

	return parseStringsWhitespaceDelimited(rest, d)
}

// parseJSON converts JSON arrays to an AST.
func parseJSON(rest string, _ *directives) (*Node, map[string]bool, error) {
	rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
	if !strings.HasPrefix(rest, "[") {
		return nil, nil, errors.Errorf(`error parsing "%s" as a JSON array`, rest)
	}

	var myJSON []interface{}
	if err := json.NewDecoder(strings.NewReader(rest)).Decode(&myJSON); err != nil {
		return nil, nil, err
	}

	var top, prev *Node
	for _, str := range myJSON {
		s, ok := str.(string)
		if !ok {
			return nil, nil, errDockerfileNotStringArray
		}

		node := &Node{Value: s}
		if prev == nil {
			top = node
		} else {
			prev.Next = node
		}
		prev = node
	}

	return top, map[string]bool{"json": true}, nil
}

// parseMaybeJSON determines if the argument appears to be a JSON array. If
// so, passes to parseJSON; if not, quotes the result and returns a single
// node.
func parseMaybeJSON(rest string, d *directives) (*Node, map[string]bool, error) {
	if rest == "" {
		return nil, nil, nil
	}

	node, attrs, err := parseJSON(rest, d)

	if err == nil {
		return node, attrs, nil
	}
	if errors.Is(err, errDockerfileNotStringArray) {
		return nil, nil, err
	}

	node = &Node{}
	node.Value = rest
	return node, nil, nil
}
