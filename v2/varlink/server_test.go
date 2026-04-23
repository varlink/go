package varlink

import (
	"context"
	"errors"
	"testing"
)

func TestServerBuilderRegisterHelpers(t *testing.T) {
	builder := NewServerBuilder(ServerConfig{})

	unary := func(context.Context, UnaryCall) error { return nil }
	stream := func(context.Context, StreamCall) error { return nil }
	upgrade := func(context.Context, UpgradeCall) error { return nil }

	if err := builder.RegisterUnary("org.example.test", "Ping", unary); err != nil {
		t.Fatalf("RegisterUnary() error = %v", err)
	}
	if err := builder.RegisterStream("org.example.test", "Watch", stream); err != nil {
		t.Fatalf("RegisterStream() error = %v", err)
	}
	if err := builder.RegisterUpgrade("org.example.test", "Bridge", upgrade); err != nil {
		t.Fatalf("RegisterUpgrade() error = %v", err)
	}

	server := builder.Build()

	got, err := server.Lookup("org.example.test.Ping")
	if err != nil {
		t.Fatalf("Lookup(unary) error = %v", err)
	}
	if got.kind != handlerKindUnary || got.unary == nil {
		t.Fatalf("Lookup(unary) returned wrong handler kind: %+v", got)
	}

	got, err = server.Lookup("org.example.test.Watch")
	if err != nil {
		t.Fatalf("Lookup(stream) error = %v", err)
	}
	if got.kind != handlerKindStream || got.stream == nil {
		t.Fatalf("Lookup(stream) returned wrong handler kind: %+v", got)
	}

	got, err = server.Lookup("org.example.test.Bridge")
	if err != nil {
		t.Fatalf("Lookup(upgrade) error = %v", err)
	}
	if got.kind != handlerKindUpgrade || got.upgrade == nil {
		t.Fatalf("Lookup(upgrade) returned wrong handler kind: %+v", got)
	}
}

func TestServerBuilderRejectsDuplicateMethods(t *testing.T) {
	builder := NewServerBuilder(ServerConfig{})
	unary := func(context.Context, UnaryCall) error { return nil }
	stream := func(context.Context, StreamCall) error { return nil }

	if err := builder.RegisterUnary("org.example.test", "Ping", unary); err != nil {
		t.Fatalf("RegisterUnary() error = %v", err)
	}
	if err := builder.RegisterUnary("org.example.test", "Ping", unary); !errors.Is(err, ErrDuplicateMethod) {
		t.Fatalf("RegisterUnary() duplicate error = %v, want %v", err, ErrDuplicateMethod)
	}
	if err := builder.RegisterStream("org.example.test", "Ping", stream); !errors.Is(err, ErrDuplicateMethod) {
		t.Fatalf("RegisterStream() duplicate error = %v, want %v", err, ErrDuplicateMethod)
	}
}

func TestServerBuilderRejectsNilHandlers(t *testing.T) {
	builder := NewServerBuilder(ServerConfig{})

	var unary UnaryHandler
	if err := builder.RegisterUnary("org.example.test", "Ping", unary); !errors.Is(err, ErrNilHandler) {
		t.Fatalf("RegisterUnary() nil error = %v, want %v", err, ErrNilHandler)
	}

	set := HandlerSet{
		Unary: map[string]UnaryHandler{
			"Ping": nil,
		},
	}
	if err := builder.Register("org.example.test", set); !errors.Is(err, ErrNilHandler) {
		t.Fatalf("Register() nil error = %v, want %v", err, ErrNilHandler)
	}
}

func TestServerBuilderRejectsDuplicateInterface(t *testing.T) {
	builder := NewServerBuilder(ServerConfig{})
	set := HandlerSet{
		Unary: map[string]UnaryHandler{
			"Ping": func(context.Context, UnaryCall) error { return nil },
		},
	}

	if err := builder.Register("org.example.test", set); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := builder.Register("org.example.test", set); !errors.Is(err, ErrDuplicateInterface) {
		t.Fatalf("Register() duplicate error = %v, want %v", err, ErrDuplicateInterface)
	}
}

func TestServerBuilderBuildClonesHandlers(t *testing.T) {
	original := HandlerSet{
		Unary: map[string]UnaryHandler{
			"Ping": func(context.Context, UnaryCall) error { return nil },
		},
	}

	builder := NewServerBuilder(ServerConfig{})
	if err := builder.Register("org.example.test", original); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	server := builder.Build()

	original.Unary["Mutated"] = func(context.Context, UnaryCall) error { return nil }
	if _, err := server.Lookup("org.example.test.Mutated"); !errors.Is(err, ErrMethodNotFound) {
		t.Fatalf("Lookup(mutated original) error = %v, want %v", err, ErrMethodNotFound)
	}

	if err := builder.RegisterUnary("org.example.test", "BuilderOnly", func(context.Context, UnaryCall) error { return nil }); err != nil {
		t.Fatalf("RegisterUnary() error = %v", err)
	}
	if _, err := server.Lookup("org.example.test.BuilderOnly"); !errors.Is(err, ErrMethodNotFound) {
		t.Fatalf("Lookup(builder mutation) error = %v, want %v", err, ErrMethodNotFound)
	}
}

func TestServerLookupErrors(t *testing.T) {
	builder := NewServerBuilder(ServerConfig{})
	if err := builder.RegisterUnary("org.example.test", "Ping", func(context.Context, UnaryCall) error { return nil }); err != nil {
		t.Fatalf("RegisterUnary() error = %v", err)
	}
	server := builder.Build()

	if _, err := server.Lookup("Ping"); !errors.Is(err, ErrInvalidMethod) {
		t.Fatalf("Lookup(invalid) error = %v, want %v", err, ErrInvalidMethod)
	}
	if _, err := server.Lookup("org.example.missing.Ping"); !errors.Is(err, ErrInterfaceNotFound) {
		t.Fatalf("Lookup(missing interface) error = %v, want %v", err, ErrInterfaceNotFound)
	}
	if _, err := server.Lookup("org.example.test.Missing"); !errors.Is(err, ErrMethodNotFound) {
		t.Fatalf("Lookup(missing method) error = %v, want %v", err, ErrMethodNotFound)
	}
}

func TestServerStartStop(t *testing.T) {
	server := NewServerBuilder(ServerConfig{}).Build()

	if server.IsStarted() {
		t.Fatal("IsStarted() = true before Start()")
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !server.IsStarted() {
		t.Fatal("IsStarted() = false after Start()")
	}
	if err := server.Start(); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("Start() second error = %v, want %v", err, ErrAlreadyStarted)
	}
	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if server.IsStarted() {
		t.Fatal("IsStarted() = true after Stop()")
	}
}
