package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/varlink/go-varlink"
)

func help(name string) {
	fmt.Printf("Usage: %s <package> <file>\n", name)
	os.Exit(1)
}

func writeTypeString(b *bytes.Buffer, t *varlink.Type) {
	switch t.Kind {
	case varlink.Bool:
		b.WriteString("bool")

	case varlink.Int:
		b.WriteString("int64")

	case varlink.Float:
		b.WriteString("float64")

	case varlink.String, varlink.Enum:
		b.WriteString("string")

	case varlink.Array:
		b.WriteString("[]")
		writeTypeString(b, t.ElementType)

	case varlink.Alias:
		b.WriteString(t.Alias)

	case varlink.Struct:
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

func writeType(b *bytes.Buffer, name string, t *varlink.Type) {
	if len(t.Fields) == 0 {
		return
	}

	b.WriteString("type " + name + " struct {\n")
	for _, field := range t.Fields {
		name := strings.Title(field.Name)
		b.WriteString("\t" + name + " ")
		writeTypeString(b, field.Type)
		b.WriteString(" `json:\"" + field.Name)

		switch field.Type.Kind {
		case varlink.Struct, varlink.String, varlink.Enum, varlink.Array:
			b.WriteString(",omitempty")
		}

		b.WriteString("\"`\n")
	}
	b.WriteString("}\n\n")
}

func main() {
	if len(os.Args) < 2 {
		help(os.Args[0])
	}

	varlinkFile := os.Args[1]

	file, err := ioutil.ReadFile(varlinkFile)
	if err != nil {
		fmt.Printf("Error reading file '%s': %s\n", varlinkFile, err)
	}

	description := strings.TrimRight(string(file), "\n")
	idl, err := varlink.NewIDL(description)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	pkgname := strings.Replace(idl.Name, ".", "", -1)

	var b bytes.Buffer
	b.WriteString("// Generated with varlink-generator -- https://github.com/varlink/go-varlink\n\n")
	b.WriteString("package " + pkgname + "\n\n")
	b.WriteString(`import "github.com/varlink/go-varlink"` + "\n\n")

	for _, member := range idl.Members {
		switch member.(type) {
		case *varlink.IDLType:
			alias := member.(*varlink.IDLType)
			writeType(&b, alias.Name, alias.Type)

		case *varlink.IDLMethod:
			method := member.(*varlink.IDLMethod)
			writeType(&b, method.Name+"_In", method.In)
			writeType(&b, method.Name+"_Out", method.Out)

		case *varlink.IDLError:
			err := member.(*varlink.IDLError)
			writeType(&b, err.Name+"_Error", err.Type)
		}
	}

	b.WriteString("func NewInterfaceDefinition() varlink.InterfaceDefinition {\n" +
		"\treturn varlink.InterfaceDefinition{\n" +
		"\t\tName:        `" + idl.Name + "`,\n" +
		"\t\tDescription: `" + idl.Description + "`,\n" +
		"\t\tMethods: map[string]struct{}{\n")
	for _, member := range idl.Members {
		switch member.(type) {
		case *varlink.IDLMethod:
			method := member.(*varlink.IDLMethod)
			b.WriteString("\t\t\t\"" + method.Name + `": {},` + "\n")
		}
	}
	b.WriteString("\t\t},\n\t}\n}\n")

	filename := path.Dir(varlinkFile) + "/" + pkgname + ".go"
	err = ioutil.WriteFile(filename, b.Bytes(), 0660)
	if err != nil {
		fmt.Printf("Error writing file '%s': %s\n", filename, err)
	}
}
