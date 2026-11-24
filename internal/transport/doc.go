// Package transport is the HTTP surface — the adapter that turns requests
// into calls on the application layer and their results into problems and
// documents. server owns the engine and the route table; one folder per
// resource keeps each handler with its paths, validation and tests; respond
// is the shared answer shape. Nothing here holds business logic: a handler
// parses, calls a service, and writes the reply.
package transport
