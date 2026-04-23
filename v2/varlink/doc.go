/*
Package varlink contains the public API for the v2 runtime.

The v2 design favors explicit call shapes over mutable request state:

  - unary calls return exactly one reply
  - stream calls return a receiver object
  - upgrades return a dedicated upgraded connection
  - pipelined unary/oneway calls use Batch

Transport policy is configured on clients and servers, while per-call
lifetimes continue to use context.Context.
*/
package varlink
