# go/varlink v2

This module is the staging area for the next major version of `go-varlink`.

The v1 module remains at the repository root:

- `github.com/varlink/go`

The new module lives under `v2/`:

- `github.com/varlink/go/v2`

## Goals

- Keep v1 stable while the redesigned runtime evolves.
- Make streaming, upgrade, and error handling first-class API concepts.
- Separate runtime policy from wire framing and code generation.
- Add defensive defaults such as bounded frame sizes and transport-level deadlines.

## Migration Strategy

1. Stabilize the v2 runtime API in this module.
2. Port generators to emit v2-aware typed clients and servers.
3. Deprecate the legacy closure-based APIs once the generated code uses the new runtime.

## Initial Package Direction

- `varlink`: public client/server/runtime API
- `internal/codec`: framing and wire encode/decode
- `internal/transport`: connection and deadline helpers

## Interface Metadata

The v2 generator supports method-mode annotations in interface comments:

```varlink
# @mode stream
method Watch() -> (value: string)

# @mode oneway
method Notify(value: string) -> ()

# @mode upgrade
method Bridge() -> ()
```

Without an explicit `@mode`, methods default to unary request/reply.

## Batching

The v2 client exposes a batch API for pipelined unary and oneway calls:

```go
batch := client.Batch()
batch.Invoke("org.example.Ping", in, &out)
batch.Oneway("org.example.Notify", notify)
err := batch.Send(ctx)
```
