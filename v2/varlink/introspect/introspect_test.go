package introspect

import "testing"

func TestParse(t *testing.T) {
	info, err := Parse(`# Demo
interface org.example.test

# Ping once
method Ping() -> ()

# @mode stream
# Watch updates
method Watch() -> ()

# @mode oneway
method Notify() -> ()

# @mode upgrade
method Bridge() -> ()
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if info.Name != "org.example.test" {
		t.Fatalf("Name = %q", info.Name)
	}
	if got, want := len(info.Methods), 4; got != want {
		t.Fatalf("methods len = %d, want %d", got, want)
	}
	if info.Methods[0].Mode != ModeUnary {
		t.Fatalf("Ping mode = %q", info.Methods[0].Mode)
	}
	if info.Methods[1].Mode != ModeStream {
		t.Fatalf("Watch mode = %q", info.Methods[1].Mode)
	}
	if info.Methods[2].Mode != ModeOneway {
		t.Fatalf("Notify mode = %q", info.Methods[2].Mode)
	}
	if info.Methods[3].Mode != ModeUpgrade {
		t.Fatalf("Bridge mode = %q", info.Methods[3].Mode)
	}
}

func TestParseMethodModeInvalid(t *testing.T) {
	if _, err := ParseMethodMode("@mode weird"); err == nil {
		t.Fatal("ParseMethodMode() unexpectedly succeeded")
	}
}
