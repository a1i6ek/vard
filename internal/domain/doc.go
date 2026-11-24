// Package domain is the heart of the server: the business concepts as
// plain Go types, with no database, no HTTP and no use case in sight.
//
// model holds the aggregates, entities and value objects — an admin, an
// application, a login flow, a refresh token — each in its own file, with
// the rules that always hold about it (validation, sealing, hashing) next
// to the type. Everything a bounded context agrees on lives here so the
// layers around it can stay thin: application orchestrates, infrastructure
// persists, transport translates.
package domain
