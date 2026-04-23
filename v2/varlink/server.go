package varlink

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"
)

// Logger matches the subset of loggers the server needs.
type Logger interface {
	Printf(format string, v ...any)
}

// ServerConfig contains runtime policy for a server.
type ServerConfig struct {
	MaxFrameSize int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	Logger       Logger
	ErrorHandler func(context.Context, error)
}

// ServiceInfo describes the service metadata exposed through org.varlink.service.
type ServiceInfo struct {
	Vendor  string
	Product string
	Version string
	URL     string
}

// UnaryHandler processes one request and returns one reply.
type UnaryHandler func(ctx context.Context, call UnaryCall) error

// StreamHandler processes one request and emits zero or more replies.
type StreamHandler func(ctx context.Context, call StreamCall) error

// UpgradeHandler processes one request and hands ownership of the connection to the caller.
type UpgradeHandler func(ctx context.Context, call UpgradeCall) error

// UnaryCall is the server-side API for a non-streaming request.
type UnaryCall interface {
	Method() string
	Decode(out any) error
	Reply(ctx context.Context, out any) error
	ReplyError(ctx context.Context, name string, parameters any) error
	IsOneway() bool
}

// StreamCall is the server-side API for a streaming request.
type StreamCall interface {
	Method() string
	Decode(out any) error
	Send(ctx context.Context, out any) error
	Close(ctx context.Context, out any) error
	ReplyError(ctx context.Context, name string, parameters any) error
}

// UpgradeCall is the server-side API for a successful connection upgrade.
type UpgradeCall interface {
	Method() string
	Decode(out any) error
	Accept(ctx context.Context, out any) (io.ReadWriteCloser, error)
	ReplyError(ctx context.Context, name string, parameters any) error
}

// ServerBuilder collects handlers and configuration before the server is started.
type ServerBuilder struct {
	cfg          ServerConfig
	info         ServiceInfo
	services     map[string]HandlerSet
	serviceNames []string
	descriptions map[string]string
}

// HandlerSet contains the handlers registered for one varlink interface.
type HandlerSet struct {
	Unary   map[string]UnaryHandler
	Stream  map[string]StreamHandler
	Upgrade map[string]UpgradeHandler
}

type handlerKind uint8

const (
	handlerKindUnary handlerKind = iota + 1
	handlerKindStream
	handlerKindUpgrade
)

type resolvedHandler struct {
	kind    handlerKind
	unary   UnaryHandler
	stream  StreamHandler
	upgrade UpgradeHandler
}

// Server is an immutable runtime built from a ServerBuilder.
type Server struct {
	cfg          ServerConfig
	info         ServiceInfo
	services     map[string]HandlerSet
	serviceNames []string
	descriptions map[string]string

	mu      sync.Mutex
	started bool
}

// NewServerBuilder returns a builder with defensive defaults applied.
func NewServerBuilder(cfg ServerConfig) *ServerBuilder {
	if cfg.MaxFrameSize == 0 {
		cfg.MaxFrameSize = MaxFrameSizeDefault
	}
	return &ServerBuilder{
		cfg:          cfg,
		services:     make(map[string]HandlerSet),
		descriptions: map[string]string{serviceInterfaceName: serviceInterfaceDescription},
	}
}

// SetInfo updates the metadata returned by org.varlink.service.GetInfo.
func (b *ServerBuilder) SetInfo(info ServiceInfo) {
	b.info = info
}

// SetInterfaceDescription installs the description returned by GetInterfaceDescription.
func (b *ServerBuilder) SetInterfaceDescription(name, description string) error {
	if name == "" {
		return ErrInvalidInterface
	}
	b.descriptions[name] = description
	return nil
}

// Register installs handlers for one interface name.
func (b *ServerBuilder) Register(name string, set HandlerSet) error {
	if name == "" {
		return ErrInvalidInterface
	}
	if _, exists := b.services[name]; exists {
		return ErrDuplicateInterface
	}
	if err := validateHandlerSet(set); err != nil {
		return err
	}
	b.services[name] = cloneHandlerSet(set)
	b.serviceNames = append(b.serviceNames, name)
	return nil
}

// RegisterUnary installs one unary method handler.
func (b *ServerBuilder) RegisterUnary(interfaceName, method string, handler UnaryHandler) error {
	return b.registerMethod(interfaceName, method, handlerKindUnary, handler)
}

// RegisterStream installs one streaming method handler.
func (b *ServerBuilder) RegisterStream(interfaceName, method string, handler StreamHandler) error {
	return b.registerMethod(interfaceName, method, handlerKindStream, handler)
}

// RegisterUpgrade installs one upgrade method handler.
func (b *ServerBuilder) RegisterUpgrade(interfaceName, method string, handler UpgradeHandler) error {
	return b.registerMethod(interfaceName, method, handlerKindUpgrade, handler)
}

// RegisterWithDescription installs handlers and the corresponding interface description together.
func (b *ServerBuilder) RegisterWithDescription(name, description string, set HandlerSet) error {
	if err := b.Register(name, set); err != nil {
		return err
	}
	return b.SetInterfaceDescription(name, description)
}

// Build freezes the registered handler set into a Server.
func (b *ServerBuilder) Build() *Server {
	services := make(map[string]HandlerSet, len(b.services))
	for name, set := range b.services {
		services[name] = cloneHandlerSet(set)
	}
	return &Server{
		cfg:          b.cfg,
		info:         b.info,
		services:     services,
		serviceNames: append([]string(nil), b.serviceNames...),
		descriptions: cloneMap(b.descriptions),
	}
}

// Config returns the server runtime configuration.
func (s *Server) Config() ServerConfig {
	return s.cfg
}

// Info returns the configured service metadata.
func (s *Server) Info() ServiceInfo {
	return s.info
}

// Start marks the server as running.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return ErrAlreadyStarted
	}
	s.started = true
	return nil
}

func (s *Server) lookup(method string) (resolvedHandler, error) {
	interfaceName, methodName, err := splitMethod(method)
	if err != nil {
		return resolvedHandler{}, err
	}

	set, ok := s.services[interfaceName]
	if !ok {
		return resolvedHandler{}, ErrInterfaceNotFound
	}
	if handler, ok := set.Unary[methodName]; ok {
		return resolvedHandler{
			kind:  handlerKindUnary,
			unary: handler,
		}, nil
	}
	if handler, ok := set.Stream[methodName]; ok {
		return resolvedHandler{
			kind:   handlerKindStream,
			stream: handler,
		}, nil
	}
	if handler, ok := set.Upgrade[methodName]; ok {
		return resolvedHandler{
			kind:    handlerKindUpgrade,
			upgrade: handler,
		}, nil
	}
	return resolvedHandler{}, ErrMethodNotFound
}

func (b *ServerBuilder) registerMethod(interfaceName, method string, kind handlerKind, handler any) error {
	if interfaceName == "" {
		return ErrInvalidInterface
	}
	if method == "" || strings.Contains(method, ".") {
		return ErrInvalidMethod
	}

	_, existed := b.services[interfaceName]
	set := cloneHandlerSet(b.services[interfaceName])
	switch kind {
	case handlerKindUnary:
		typedHandler := handler.(UnaryHandler)
		if typedHandler == nil {
			return ErrNilHandler
		}
		if set.Unary == nil {
			set.Unary = make(map[string]UnaryHandler)
		}
		if methodExists(set, method) {
			return ErrDuplicateMethod
		}
		set.Unary[method] = typedHandler
	case handlerKindStream:
		typedHandler := handler.(StreamHandler)
		if typedHandler == nil {
			return ErrNilHandler
		}
		if set.Stream == nil {
			set.Stream = make(map[string]StreamHandler)
		}
		if methodExists(set, method) {
			return ErrDuplicateMethod
		}
		set.Stream[method] = typedHandler
	case handlerKindUpgrade:
		typedHandler := handler.(UpgradeHandler)
		if typedHandler == nil {
			return ErrNilHandler
		}
		if set.Upgrade == nil {
			set.Upgrade = make(map[string]UpgradeHandler)
		}
		if methodExists(set, method) {
			return ErrDuplicateMethod
		}
		set.Upgrade[method] = typedHandler
	default:
		return ErrInvalidMethod
	}

	b.services[interfaceName] = set
	if !existed {
		b.serviceNames = append(b.serviceNames, interfaceName)
	}
	return nil
}

func validateHandlerSet(set HandlerSet) error {
	seen := make(map[string]struct{}, len(set.Unary)+len(set.Stream)+len(set.Upgrade))
	for name := range set.Unary {
		if name == "" || strings.Contains(name, ".") {
			return ErrInvalidMethod
		}
		if set.Unary[name] == nil {
			return ErrNilHandler
		}
		seen[name] = struct{}{}
	}
	for name := range set.Stream {
		if name == "" || strings.Contains(name, ".") {
			return ErrInvalidMethod
		}
		if set.Stream[name] == nil {
			return ErrNilHandler
		}
		if _, exists := seen[name]; exists {
			return ErrDuplicateMethod
		}
		seen[name] = struct{}{}
	}
	for name := range set.Upgrade {
		if name == "" || strings.Contains(name, ".") {
			return ErrInvalidMethod
		}
		if set.Upgrade[name] == nil {
			return ErrNilHandler
		}
		if _, exists := seen[name]; exists {
			return ErrDuplicateMethod
		}
		seen[name] = struct{}{}
	}
	return nil
}

func cloneHandlerSet(set HandlerSet) HandlerSet {
	return HandlerSet{
		Unary:   cloneMap(set.Unary),
		Stream:  cloneMap(set.Stream),
		Upgrade: cloneMap(set.Upgrade),
	}
}

func cloneMap[T any](in map[string]T) map[string]T {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]T, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func methodExists(set HandlerSet, method string) bool {
	if _, ok := set.Unary[method]; ok {
		return true
	}
	if _, ok := set.Stream[method]; ok {
		return true
	}
	if _, ok := set.Upgrade[method]; ok {
		return true
	}
	return false
}

func splitMethod(method string) (string, string, error) {
	i := strings.LastIndex(method, ".")
	if i <= 0 || i == len(method)-1 {
		return "", "", ErrInvalidMethod
	}
	interfaceName := method[:i]
	methodName := method[i+1:]
	if strings.Contains(methodName, ".") {
		return "", "", ErrInvalidMethod
	}
	return interfaceName, methodName, nil
}

func (s *Server) IsStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	s.started = false
	return nil
}

func (s *Server) Handler(interfaceName, methodName string) (resolvedHandler, error) {
	return s.lookup(interfaceName + "." + methodName)
}

func (s *Server) Lookup(method string) (resolvedHandler, error) {
	return s.lookup(method)
}

func (s *Server) interfaceNames() []string {
	names := make([]string, 0, len(s.serviceNames)+1)
	names = append(names, serviceInterfaceName)
	names = append(names, s.serviceNames...)
	return names
}

func (s *Server) interfaceDescription(name string) (string, bool) {
	description, ok := s.descriptions[name]
	return description, ok
}
