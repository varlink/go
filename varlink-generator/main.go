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

func writeType(b *bytes.Buffer, t *varlink.Type) {
	switch t.Kind {
	case varlink.Bool:
		b.WriteString("bool")

	case varlink.Int:
		b.WriteString("int64")

	case varlink.Float:
		b.WriteString("float64")

	case varlink.String:
		b.WriteString("string")

	case varlink.Array:
		b.WriteString("[]")
		writeType(b, t.ElementType)

	case varlink.Alias:
		b.WriteString(t.Alias)
	}
}

func writeTypeDecl(b *bytes.Buffer, name string, t *varlink.Type) {
	if len(t.Fields) == 0 {
		return
	}

	b.WriteString("type " + name + " struct {\n")
	for _, field := range t.Fields {
		name := strings.Title(field.Name)
		b.WriteString("\t" + name + " ")
		writeType(b, field.Type)
		b.WriteString(" `json:\"" + field.Name + "\"`\n")
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
	iface := varlink.NewInterface(description)
	pkgname := strings.Replace(iface.Name, ".", "", -1)

	var b bytes.Buffer
	b.WriteString("package " + pkgname + "\n\n")
	b.WriteString(`import "github.com/varlink/go-varlink"` + "\n\n")

	for _, member := range iface.Members {
		switch member.(type) {
		case *varlink.TypeAlias:
			alias := member.(*varlink.TypeAlias)
			writeTypeDecl(&b, alias.Name, alias.Type)

		case *varlink.MethodT:
			method := member.(*varlink.MethodT)
			writeTypeDecl(&b, method.Name+"_In", method.In)
			writeTypeDecl(&b, method.Name+"_Out", method.Out)

		case *varlink.ErrorType:
			err := member.(*varlink.ErrorType)
			writeTypeDecl(&b, err.Name+"_Error", err.Type)
		}
	}

	b.WriteString("func NewInterfaceDefinition() varlink.InterfaceDefinition {\n" +
		"\treturn varlink.InterfaceDefinition{\n" +
		"\t\tName:        `" + iface.Name + "`,\n" +
		"\t\tDescription: `" + iface.Description + "`,\n" +
		"\t\tMethods: map[string]bool{\n")
	for _, member := range iface.Members {
		switch member.(type) {
		case *varlink.MethodT:
			method := member.(*varlink.MethodT)
			b.WriteString("\t\t\t\"" + method.Name + `": true,` + "\n")
		}
	}
	b.WriteString("\t\t},\n\t}\n}\n")

	filename := path.Dir(varlinkFile) + "/" + pkgname + ".go"
	err = ioutil.WriteFile(filename, b.Bytes(), 0660)
	if err != nil {
		fmt.Printf("Error writing file '%s': %s\n", filename, err)
	}
}
