package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/varlink/go/varlink/idl"
)

var goKeywords = map[string]struct{}{
	"break":       {},
	"case":        {},
	"chan":        {},
	"const":       {},
	"continue":    {},
	"default":     {},
	"defer":       {},
	"else":        {},
	"fallthrough": {},
	"for":         {},
	"func":        {},
	"go":          {},
	"goto":        {},
	"if":          {},
	"import":      {},
	"interface":   {},
	"map":         {},
	"package":     {},
	"range":       {},
	"return":      {},
	"select":      {},
	"struct":      {},
	"switch":      {},
	"type":        {},
	"var":         {},
}

func sanitizeGoName(name string) string {
	if _, ok := goKeywords[name]; !ok {
		return name
	}
	return name + "A"
}

func writeTypeString(b *bytes.Buffer, t *idl.Type, level int) {
	switch t.Kind {
	case idl.TypeBool:
		b.WriteString("bool")

	case idl.TypeInt:
		b.WriteString("int64")

	case idl.TypeFloat:
		b.WriteString("float64")

	case idl.TypeString, idl.TypeEnum:
		b.WriteString("string")

	case idl.TypeArray:
		b.WriteString("[]")
		writeTypeString(b, t.ElementType, 0)

	case idl.TypeAlias:
		b.WriteString(t.Alias)

	case idl.TypeStruct:
		if level > 0 {
			b.WriteString("struct {")
		}
		for i, field := range t.Fields {
			if i > 0 {
				if level > 0 {
					b.WriteString("; ")
				} else {
					b.WriteString(", ")
				}
			}
			b.WriteString(sanitizeGoName(field.Name) + " ")
			writeTypeString(b, field.Type, level)
		}
		if level > 0 {
			b.WriteString("}")
		}
	}
}

func writeType(b *bytes.Buffer, decl string, t *idl.Type, omit bool, ident int) {
	for i := 0; i < ident; i++ {
		b.WriteString("\t")
	}
	b.WriteString(decl + " struct {\n")
	for _, field := range t.Fields {
		name := strings.Title(field.Name)
		for i := 0; i < ident+1; i++ {
			b.WriteString("\t")
		}
		b.WriteString(name + " ")
		writeTypeString(b, field.Type, 0)
		b.WriteString(" `json:\"" + field.Name)
		if omit {
			switch field.Type.Kind {
			case idl.TypeStruct, idl.TypeString, idl.TypeEnum, idl.TypeArray, idl.TypeAlias:
				b.WriteString(",omitempty")
			}
		}
		b.WriteString("\"`\n")
	}
	for i := 0; i < ident; i++ {
		b.WriteString("\t")
	}
	b.WriteString("}\n")
}

func generateTemplate(description string) (string, []byte, error) {
	description = strings.TrimRight(description, "\n")

	midl, err := idl.New(description)
	if err != nil {
		return "", nil, err
	}

	pkgname := strings.Replace(midl.Name, ".", "", -1)

	var b bytes.Buffer
	b.WriteString("// Generated with varlink-generator -- https://github.com/varlink/go/cmd/varlink-generator\n")
	b.WriteString("package " + pkgname + "\n\n")
	b.WriteString(`import "github.com/varlink/go/varlink"` + "\n\n")

	// Type declarations
	for _, a := range midl.Aliases {
		writeType(&b, "type "+a.Name, a.Type, true, 0)
		b.WriteString("\n")
	}

	// Local interface with all methods
	b.WriteString("type " + pkgname + "Interface interface {\n")
	for _, m := range midl.Methods {
		b.WriteString("\t" + m.Name + "(c VarlinkCall")
		if len(m.In.Fields) > 0 {
			b.WriteString(", ")
			writeTypeString(&b, m.In, 0)
		}
		b.WriteString(") error\n")
	}
	b.WriteString("}\n\n")

	// Local object with all methods
	b.WriteString("type VarlinkCall struct{ varlink.Call }\n\n")

	// Reply methods for all varlink errors
	for _, e := range midl.Errors {
		b.WriteString("func (c *VarlinkCall) Reply" + e.Name + "(")
		if len(e.Type.Fields) > 0 {
			writeTypeString(&b, e.Type, 0)
		}
		b.WriteString(") error {\n")
		if len(e.Type.Fields) > 0 {
			writeType(&b, "var out", e.Type, true, 1)
			for _, field := range e.Type.Fields {
				b.WriteString("\tout." + strings.Title(field.Name) + " = " + sanitizeGoName(field.Name) + "\n")
			}
			b.WriteString("\treturn c.ReplyError(\"" + midl.Name + "." + e.Name + "\", &out)\n")
		} else {
			b.WriteString("\treturn c.ReplyError(\"" + midl.Name + "." + e.Name + "\", nil)\n")
		}
		b.WriteString("}\n\n")
	}

	// Reply methods for all varlink methods
	for _, m := range midl.Methods {
		b.WriteString("func (c *VarlinkCall) Reply" + m.Name + "(")
		if len(m.Out.Fields) > 0 {
			writeTypeString(&b, m.Out, 0)
		}
		b.WriteString(") error {\n")
		if len(m.Out.Fields) > 0 {
			writeType(&b, "var out", m.Out, true, 1)
			for _, field := range m.Out.Fields {
				b.WriteString("\tout." + strings.Title(field.Name) + " = " + sanitizeGoName(field.Name) + "\n")
			}
			b.WriteString("\treturn c.Reply(&out)\n")
		} else {
			b.WriteString("\treturn c.Reply(nil)\n")
		}
		b.WriteString("}\n\n")
	}

	// Dummy methods for all varlink methods
	for _, m := range midl.Methods {
		b.WriteString("func (s *VarlinkInterface) " + m.Name + "(c VarlinkCall")
		if len(m.In.Fields) > 0 {
			b.WriteString(", ")
			writeTypeString(&b, m.In, 0)
		}
		b.WriteString(") error {\n" +
			"\treturn c.ReplyMethodNotImplemented(\"" + m.Name + "\")\n" +
			"}\n\n")
	}

	// Method call dispatcher
	b.WriteString("func (s *VarlinkInterface) VarlinkDispatch(call varlink.Call, methodname string) error {\n" +
		"\tswitch methodname {\n")
	for _, m := range midl.Methods {
		b.WriteString("\tcase \"" + m.Name + "\":\n")
		if len(m.In.Fields) > 0 {
			writeType(&b, "var in", m.In, false, 2)
			b.WriteString("\t\terr := call.GetParameters(&in)\n" +
				"\t\tif err != nil {\n" +
				"\t\t\treturn call.ReplyInvalidParameter(\"parameters\")\n" +
				"\t\t}\n")
			b.WriteString("\t\treturn s." + pkgname + "Interface." + m.Name + "(VarlinkCall{call}")
			if len(m.In.Fields) > 0 {
				for _, field := range m.In.Fields {
					b.WriteString(", in." + strings.Title(field.Name))
				}
			}
			b.WriteString(")\n")
		} else {
			b.WriteString("\t\treturn s." + pkgname + "Interface." + m.Name + "(VarlinkCall{call})\n")
		}
	}
	b.WriteString("\n" +
		"\tdefault:\n" +
		"\t\treturn call.ReplyMethodNotFound(methodname)\n" +
		"\t}\n" +
		"}\n")

	// Varlink interface name
	b.WriteString("func (s *VarlinkInterface) VarlinkGetName() string {\n" +
		"\treturn `" + midl.Name + "`\n" + "}\n\n")

	// Varlink interface description
	b.WriteString("func (s *VarlinkInterface) VarlinkGetDescription() string {\n" +
		"\treturn `" + midl.Description + "`\n}\n\n")

	b.WriteString("type VarlinkInterface struct {\n" +
		"\t" + pkgname + "Interface\n" +
		"}\n\n")

	b.WriteString("func VarlinkNew(m " + pkgname + "Interface) *VarlinkInterface {\n" +
		"\treturn &VarlinkInterface{m}\n" +
		"}\n")

	return pkgname, b.Bytes(), nil
}

func generateFile(varlinkFile string) {
	file, err := ioutil.ReadFile(varlinkFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file '%s': %s\n", varlinkFile, err)
		os.Exit(1)
	}

	pkgname, b, err := generateTemplate(string(file))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing file '%s': %s\n", varlinkFile, err)
		os.Exit(1)
	}

	filename := path.Dir(varlinkFile) + "/" + pkgname + ".go"
	err = ioutil.WriteFile(filename, b, 0660)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file '%s': %s\n", filename, err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <file>\n", os.Args[0])
		os.Exit(1)
	}
	generateFile(os.Args[1])
}
