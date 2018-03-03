package varlink

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	Bool = iota
	Int
	Float
	String
	Array
	Struct
	Enum
	Alias
)

type TypeKind uint

type Type struct {
	Kind        TypeKind
	ElementType *Type
	Alias       string
	Fields      []TypeField
}

type TypeField struct {
	Name string
	Type *Type
}

type Idl struct {
	Name        string
	Doc         string
	Description string
	Members     []interface{}
	Aliases     map[string]*IdlType
	Methods     map[string]*IdlMethod
	Errors      map[string]*IdlError
}

type IdlType struct {
	Name string
	Doc  string
	Type *Type
}

type IdlMethod struct {
	Name string
	Doc  string
	In   *Type
	Out  *Type
}

type IdlError struct {
	Name string
	Type *Type
}

type parser struct {
	input       string
	position    int
	lineStart   int
	lastComment bytes.Buffer
}

func (p *parser) next() int {
	r := -1

	if p.position < len(p.input) {
		r = int(p.input[p.position])
	}

	p.position += 1
	return r
}

func (p *parser) backup() {
	p.position -= 1
}

func (p *parser) advance() bool {
	for {
		char := p.next()

		if char == '\n' {
			p.lineStart = p.position
			p.lastComment.Reset()

		} else if char == ' ' || char == '\t' {
			// ignore

		} else if char == '#' {
			p.next()
			start := p.position
			for {
				c := p.next()
				if c < 0 || c == '\n' {
					p.backup()
					break
				}
			}
			if p.lastComment.Len() > 0 {
				p.lastComment.WriteByte('\n')
			}
			p.lastComment.WriteString(p.input[start:p.position])
			p.next()

		} else {
			p.backup()
			break
		}
	}

	return p.position < len(p.input)
}

func (p *parser) advanceOnLine() {
	for {
		char := p.next()
		if char != ' ' {
			p.backup()
			return
		}
	}
}

func (p *parser) readKeyword() string {
	start := p.position

	for {
		char := p.next()
		if char < 'a' || char > 'z' {
			p.backup()
			break
		}
	}

	return p.input[start:p.position]
}

func (p *parser) readInterfaceName() string {
	start := p.position

	for {
		char := p.next()
		if (char < 'a' || char > 'z') && char != '-' && char != '.' {
			p.backup()
			break
		}
	}

	name := p.input[start:p.position]
	if len(name) < 3 || len(name) > 255 {
		return ""
	}

	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return ""
	}

	for _, part := range parts {
		if len(part) == 0 || strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return ""
		}
	}

	return name
}

func (p *parser) readFieldName() string {
	start := p.position

	char := p.next()
	if (char < 'a' || char > 'z') && char != '_' {
		p.backup()
		return ""
	}

	for {
		char := p.next()
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			p.backup()
			break
		}
	}

	return p.input[start:p.position]
}

func (p *parser) readTypeName() string {
	start := p.position

	for {
		char := p.next()
		if (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			p.backup()
			break
		}
	}

	return p.input[start:p.position]
}

func (p *parser) readStructType() *Type {
	if p.next() != '(' {
		p.backup()
		return nil
	}

	t := &Type{Kind: Struct}
	t.Fields = make([]TypeField, 0)

	char := p.next()
	if char != ')' {
		p.backup()

		for {
			field := TypeField{}

			p.advance()
			field.Name = p.readFieldName()
			if field.Name == "" {
				return nil
			}

			p.advance()

			// Enums have no types, they are just a list of names
			if p.next() == ':' {
				if t.Kind == Enum {
					return nil
				}

				p.advance()
				field.Type = p.readType()
				if field.Type == nil {
					return nil
				}

			} else {
				t.Kind = Enum
				p.backup()
			}

			t.Fields = append(t.Fields, field)

			p.advance()
			char = p.next()
			if char != ',' {
				break
			}
		}

		if char != ')' {
			return nil
		}
	}

	return t
}

func (p *parser) readType() *Type {
	var t *Type

	if keyword := p.readKeyword(); keyword != "" {
		switch keyword {
		case "bool":
			t = &Type{Kind: Bool}

		case "int":
			t = &Type{Kind: Int}

		case "float":
			t = &Type{Kind: Float}

		case "string":
			t = &Type{Kind: String}
		}

	} else if name := p.readTypeName(); name != "" {
		t = &Type{Kind: Alias, Alias: name}

	} else if t = p.readStructType(); t == nil {
		return nil
	}

	if p.next() == '[' {
		if p.next() != ']' {
			return nil
		}
		t = &Type{Kind: Array, ElementType: t}

	} else {
		p.backup()
	}

	return t
}

func (p *parser) readIdl() (*Idl, error) {
	if keyword := p.readKeyword(); keyword != "interface" {
		return nil, fmt.Errorf("missing interface keyword")
	}

	idl := &Idl{
		Members: make([]interface{}, 0),
		Aliases: make(map[string]*IdlType),
		Methods: make(map[string]*IdlMethod),
		Errors:  make(map[string]*IdlError),
	}

	p.advance()
	idl.Doc = p.lastComment.String()
	idl.Name = p.readInterfaceName()
	if idl.Name == "" {
		return nil, fmt.Errorf("interface name")
	}

	for {
		if !p.advance() {
			break
		}

		switch keyword := p.readKeyword(); keyword {
		case "type":
			alias := &IdlType{}

			p.advance()
			alias.Doc = p.lastComment.String()
			alias.Name = p.readTypeName()
			if alias.Name == "" {
				return nil, fmt.Errorf("missing alias name")
			}

			p.advance()
			alias.Type = p.readType()
			if alias.Type == nil {
				return nil, fmt.Errorf("missing alias type")
			}

			idl.Members = append(idl.Members, alias)
			idl.Aliases[alias.Name] = alias

		case "method":
			method := &IdlMethod{}

			p.advance()
			method.Doc = p.lastComment.String()
			method.Name = p.readTypeName()
			if method.Name == "" {
				return nil, fmt.Errorf("missing method type")
			}

			p.advance()
			method.In = p.readType()
			if method.In == nil {
				return nil, fmt.Errorf("missing method input")
			}

			p.advance()
			one := p.next()
			two := p.next()
			if (one != '-') || two != '>' {
				return nil, fmt.Errorf("missing method '->' operator")
			}

			p.advance()
			method.Out = p.readType()
			if method.Out == nil {
				return nil, fmt.Errorf("missing method output")
			}

			idl.Members = append(idl.Members, method)
			idl.Methods[method.Name] = method

		case "error":
			err := &IdlError{}

			p.advance()
			err.Name = p.readTypeName()
			if err.Name == "" {
				return nil, fmt.Errorf("missing error name")
			}

			p.advanceOnLine()
			err.Type = p.readType()

			idl.Members = append(idl.Members, err)
			idl.Errors[err.Name] = err

		default:
			return nil, fmt.Errorf("unknown keyword '%s'", keyword)
		}
	}

	return idl, nil
}

func NewIdl(description string) (*Idl, error) {
	p := &parser{input: description}

	p.advance()
	idl, err := p.readIdl()
	if err != nil {
		return nil, err
	}

	if p.advance() {
		return nil, fmt.Errorf("advance error %s", p.input[p.position:])
	}

	idl.Description = description
	return idl, nil
}
