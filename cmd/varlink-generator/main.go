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

func writeTypeString(b *bytes.Buffer, t *idl.Type) {
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
		writeTypeString(b, t.ElementType)

	case idl.TypeAlias:
		b.WriteString(t.Alias + "_T")

	case idl.TypeStruct:
		b.WriteString("struct {")
		for i, field := range t.Fields {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(field.Name + " ")
			writeTypeString(b, field.Type)
		}
		b.WriteString("}")
	}
}

func writeType(b *bytes.Buffer, name string, omitempty bool, t *idl.Type) {
	if len(t.Fields) == 0 {
		return
	}

	b.WriteString("type " + name + " struct {\n")
	for _, field := range t.Fields {
		name := strings.Title(field.Name)
		b.WriteString("\t" + name + " ")
		writeTypeString(b, field.Type)
		b.WriteString(" `json:\"" + field.Name)

		if omitempty {
			switch field.Type.Kind {
			case idl.TypeStruct, idl.TypeString, idl.TypeEnum, idl.TypeArray, idl.TypeAlias:
				b.WriteString(",omitempty")
			}
		}

		b.WriteString("\"`\n")
	}
	b.WriteString("}\n\n")
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <file>\n", os.Args[0])
		os.Exit(1)
	}

	varlinkFile := os.Args[1]

	file, err := ioutil.ReadFile(varlinkFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file '%s': %s\n", varlinkFile, err)
		os.Exit(1)
	}

	description := strings.TrimRight(string(file), "\n")
	midl, err := idl.New(description)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing file '%s': %s\n", varlinkFile, err)
		os.Exit(1)
	}
	pkgname := strings.Replace(midl.Name, ".", "", -1)

	var b bytes.Buffer
	b.WriteString("// Generated with varlink-generator -- https://github.com/varlink/go/cmd/varlink-generator\n\n")
	b.WriteString("package " + pkgname + "\n\n")
	b.WriteString(`import "github.com/varlink/go/varlink"` + "\n\n")

	for _, member := range midl.Members {
		switch member.(type) {
		case *idl.Alias:
			a := member.(*idl.Alias)
			writeType(&b, a.Name+"_T", true, a.Type)

		case *idl.Method:
			m := member.(*idl.Method)
			writeType(&b, m.Name+"_In", false, m.In)
			writeType(&b, m.Name+"_Out", true, m.Out)

		case *idl.Error:
			e := member.(*idl.Error)
			writeType(&b, e.Name+"_Error", true, e.Type)
		}
	}

	b.WriteString("func New() varlink.Interface {\n" +
		"\treturn varlink.Interface{\n" +
		"\t\tName:        `" + midl.Name + "`,\n" +
		"\t\tDescription: `" + midl.Description + "`,\n" +
		"\t\tMethods: varlink.MethodMap{\n")
	for m := range midl.Methods {
		b.WriteString("\t\t\t\"" + m + `": nil,` + "\n")
	}
	b.WriteString("\t\t},\n\t}\n}\n")

	filename := path.Dir(varlinkFile) + "/" + pkgname + ".go"
	err = ioutil.WriteFile(filename, b.Bytes(), 0660)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file '%s': %s\n", filename, err)
		os.Exit(1)
	}
}
