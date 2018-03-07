package main

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func expect(t *testing.T, expected string, returned string) {
	if strings.Compare(returned, expected) != 0 {
		t.Fatalf("Expected(%d): `%s`\nGot(%d): `%s`\n",
			len(expected), expected,
			len(returned), returned)
	}
}

func TestIDLParser(t *testing.T) {
	pkgname, b, err := generateTemplate(`
# Interface to jump a spacecraft to another point in space. The 
# FTL Drive is the propulsion system to achieve faster-than-light
# travel through space. A ship making a properly calculated
# jump can arrive safely in planetary orbit, or alongside other
# ships or spaceborne objects.
interface org.example.ftl

# The current state of the FTL drive and the amount of fuel
# available to jump.
type DriveCondition (
  state: (idle, spooling, busy),
  tylium_level: int
)

# Speed, trajectory and jump duration is calculated prior to
# activating the FTL drive.
type DriveConfiguration (
  speed: int,
  trajectory: int,
  duration: int
)

# The galactic coordinates use the Sun as the origin. Galactic
# longitude is measured with primary direction from the Sun to
# the center of the galaxy in the galactic plane, while the
# galactic latitude measures the angle of the object above the
# galactic plane.
type Coordinate (
  longitude: float,
  latitude: float,
  distance: int
)

# Monitor the drive. The method will reply with an update whenever
# the drive's state changes
method Monitor() -> (condition: DriveCondition)

# Calculate the drive's jump parameters from the current
# position to the target position in the galaxy
method CalculateConfiguration(
  current: Coordinate,
  target: Coordinate
) -> (configuration: DriveConfiguration)

# Jump to the calculated point in space
method Jump(configuration: DriveConfiguration) -> ()

# There is not enough tylium to jump with the given parameters
error NotEnoughEnergy ()

# The supplied parameters are outside the supported range
error ParameterOutOfRange (field: string)
	`)

	if err != nil {
		t.Fatalf("Error parsing %v", err)
	}
	expect(t, "orgexampleftl", pkgname)
	if len(b) <= 0 {
		t.Fatal("No generated go source")
	}
}

func testParse(t *testing.T, pass bool, description string) {
	_, _, line, _ := runtime.Caller(1)
	t.Run(fmt.Sprintf("Line-%d", line), func(t *testing.T) {

		pkgname, b, err := generateTemplate(description)
		if pass {
			if err != nil {
				t.Fatalf("generateTemplate(`%s`): %v", description, err)
			}
			if len(pkgname) <= 0 {
				t.Fatalf("generateTemplate(`%s`): returned no pkgname", description)
			}
			if len(b) <= 0 {
				t.Fatalf("generateTemplate(`%s`): returned no go source", description)
			}
		}
		if !pass && (err == nil) {
			t.Fatalf("generateTemplate(`%s`): did not fail", description)
		}
	})
}

func TestOneMethod(t *testing.T) {
	testParse(t, true, "interface foo.bar\nmethod Foo()->()")
}

func TestOneMethodNoType(t *testing.T) {
	testParse(t, false, "interface foo.bar\nmethod Foo()->(b:)")
}

func TestDomainNames(t *testing.T) {
	testParse(t, true, "interface org.varlink.service\nmethod F()->()")
	testParse(t, true, "interface com.example.0example\nmethod F()->()")
	testParse(t, true, "interface com.example.example-dash\nmethod F()->()")
	testParse(t, true, "interface xn--lgbbat1ad8j.example.algeria\nmethod F()->()")
	testParse(t, false, "interface com.-example.leadinghyphen\nmethod F()->()")
	testParse(t, false, "interface com.example-.danglinghyphen-\nmethod F()->()")
	testParse(t, false, "interface Com.example.uppercase-toplevel\nmethod F()->()")
	testParse(t, false, "interface Co9.example.number-toplevel\nmethod F()->()")
	testParse(t, false, "interface 1om.example.number-toplevel\nmethod F()->()")
	testParse(t, false, "interface com.Example\nmethod F()->()")
	var name string
	for i := 0; i < 255; i++ {
		name += "a"
	}
	testParse(t, false, "interface com.example.toolong"+name+"\nmethod F()->()")
	testParse(t, false, "interface xn--example.toolong"+name+"\nmethod F()->()")
}

func TestNoMethod(t *testing.T) {
	testParse(t, false, `
interface org.varlink.service
  type Interface (name: string, types: Type[], methods: Method[])
  type Property (key: string, value: string)
`)
}

func TestTypeNoArgs(t *testing.T) {
	testParse(t, true, "interface foo.bar\n type I ()\nmethod F()->()")
}

func TestTypeOneArg(t *testing.T) {
	testParse(t, true, "interface foo.bar\n type I (b:bool)\nmethod F()->()")
}

func TestTypeOneArray(t *testing.T) {
	testParse(t, true, "interface foo.bar\n type I (b:bool[])\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (b:bool[ ])\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (b:bool[1])\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (b:bool[ 1 ])\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (b:bool[ 1 1 ])\nmethod  F()->()")
}

func TestFieldnames(t *testing.T) {
	testParse(t, false, "interface foo.bar\n type I (Test:bool[])\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (_test:bool[])\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (Äest:bool[])\nmethod  F()->()")
}

func TestEnum(t *testing.T) {
	testParse(t, true, "interface foo.bar\n type I (b:(foo, bar, baz))\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (foo, bar, baz : bool)\nmethod  F()->()")
}

func TestIncomplete(t *testing.T) {
	testParse(t, false, "interfacef foo.bar\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\nmethod  F()->()\ntype I (b: bool")
	testParse(t, false, "interface foo.bar\nmethod  F()->(")
	testParse(t, false, "interface foo.bar\nmethod  F(")
	testParse(t, false, "interface foo.bar\nmethod  ()->()")
	testParse(t, false, "interface foo.bar\nmethod  F->()\n")
	testParse(t, false, "interface foo.bar\nmethod  F()->\n")
	testParse(t, false, "interface foo.bar\nmethod  F()>()\n")
	testParse(t, false, "interface foo.bar\nmethod  F()->()\ntype (b: bool)")
	testParse(t, false, "interface foo.bar\nmethod  F()->()\nerror (b: bool)")
	testParse(t, false, "interface foo.bar\nmethod  F()->()\n dfghdrg")
}

func TestDuplicate(t *testing.T) {
	testParse(t, false, `
interface foo.example
	type Device()
	type Device()
	type T()
	type T()
	method F() -> ()
	method F() -> ()
`)
}
