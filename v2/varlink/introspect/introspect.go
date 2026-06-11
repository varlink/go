package introspect

import (
	"bufio"
	"fmt"
	"strings"
)

type MethodMode string

const (
	ModeUnary   MethodMode = "unary"
	ModeStream  MethodMode = "stream"
	ModeOneway  MethodMode = "oneway"
	ModeUpgrade MethodMode = "upgrade"
)

type MethodInfo struct {
	Name string
	Doc  string
	Mode MethodMode
}

type InterfaceInfo struct {
	Name        string
	Description string
	Methods     []MethodInfo
}

func Parse(description string) (*InterfaceInfo, error) {
	info := &InterfaceInfo{Description: strings.TrimRight(description, "\n")}
	scanner := bufio.NewScanner(strings.NewReader(description))
	var pendingDoc []string

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "#"):
			docLine := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			pendingDoc = append(pendingDoc, docLine)
		case strings.HasPrefix(trimmed, "interface "):
			fields := strings.Fields(trimmed)
			if len(fields) < 2 {
				return nil, fmt.Errorf("introspect: malformed interface line %q", line)
			}
			info.Name = fields[1]
			pendingDoc = nil
		case strings.HasPrefix(trimmed, "method "):
			name, err := parseMethodName(trimmed)
			if err != nil {
				return nil, err
			}
			doc := strings.Join(pendingDoc, "\n")
			mode, err := ParseMethodMode(doc)
			if err != nil {
				return nil, fmt.Errorf("introspect: method %s: %w", name, err)
			}
			info.Methods = append(info.Methods, MethodInfo{
				Name: name,
				Doc:  doc,
				Mode: mode,
			})
			pendingDoc = nil
		case strings.HasPrefix(trimmed, "type "), strings.HasPrefix(trimmed, "error "):
			pendingDoc = nil
		case trimmed == "":
			// Keep doc comments attached across blank lines, matching typical IDL formatting.
		default:
			pendingDoc = nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if info.Name == "" {
		return nil, fmt.Errorf("introspect: missing interface name")
	}
	return info, nil
}

func ParseMethodMode(doc string) (MethodMode, error) {
	mode := ModeUnary
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "@mode" {
			continue
		}
		if len(fields) != 2 {
			return "", fmt.Errorf("invalid @mode directive %q", line)
		}
		switch MethodMode(fields[1]) {
		case ModeUnary, ModeStream, ModeOneway, ModeUpgrade:
			mode = MethodMode(fields[1])
		default:
			return "", fmt.Errorf("unknown method mode %q", fields[1])
		}
	}
	return mode, nil
}

func parseMethodName(line string) (string, error) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "method "))
	idx := strings.IndexByte(line, '(')
	if idx <= 0 {
		return "", fmt.Errorf("introspect: malformed method line %q", line)
	}
	return strings.TrimSpace(line[:idx]), nil
}
