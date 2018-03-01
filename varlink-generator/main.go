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
	}
}

func writeTypeDecl(b *bytes.Buffer, name string, t *varlink.Type) {
	if len(t.Fields) == 0 {
		return
	}

	b.WriteString("type " + name + " {\n")
	for _, field := range t.Fields {
		name := strings.Title(field.Name)
		b.WriteString("\t" + name + " ")
		writeType(b, field.Type)
		b.WriteString(" `json:\"" + field.Name + "\"`\n")
	}
	b.WriteString("}\n\n")
}

func main() {
	if len(os.Args) < 3 {
		help(os.Args[0])
	}

	file, err := ioutil.ReadFile(os.Args[2])
	if err != nil {
		fmt.Printf("Error reading file '%s': %s\n", os.Args[2], err)
	}

	intf := strings.TrimRight(string(file), "\n")
	iface := varlink.NewInterface(intf)

	var b bytes.Buffer

	iname := strings.Replace(iface.Name, ".", "", -1)
	b.WriteString("package " + iname + "\n\n")

	for _, member := range iface.Members {
		switch member.(type) {
		case *varlink.TypeAlias:
			alias := member.(*varlink.TypeAlias)
			writeTypeDecl(&b, alias.Name, alias.Type)

		case *varlink.MethodT:
			method := member.(*varlink.MethodT)
			writeTypeDecl(&b, method.Name + "_CallParameters", method.In)
			writeTypeDecl(&b, method.Name + "_ReplyParameters", method.Out)

		case *varlink.ErrorType:
			err := member.(*varlink.ErrorType)
			writeTypeDecl(&b, err.Name + "_ErrorParameters", err.Type)
		}
	}
	fmt.Println(b.String())

	pkg := os.Args[1]
	name := path.Base(os.Args[2])
	dir := path.Dir(os.Args[2])

	// Convert input file interface name to CamelCase
	name = strings.TrimSuffix(name, ".varlink")
	name = strings.Replace(name, ".", " ", -1)
	name = strings.Title(name)
	name = strings.Replace(name, " ", "", -1)

	out := "package " + pkg + "\n\n"
	out += "var " + name + " = \n"
	out += "`" + intf + "\n`"
	out += "\n"
	filename := dir + "/" + name + ".go"
	err = ioutil.WriteFile(filename, []byte(out), 0660)
	if err != nil {
		fmt.Printf("Error reading file '%s': %s\n", filename, err)
	}
}
