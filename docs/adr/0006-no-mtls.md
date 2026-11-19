# 0006 No mutual TLS client authentication, for now

- **Status:** Accepted (2026-11-19)
- **Date:** 2026-11-19

## Context

RFC 8705 lets a client authenticate with a TLS client certificate
(`tls_client_auth`, `self_signed_tls_client_auth`), and binds its tokens to
that certificate (`cnf.x5t#S256`). The plan names it as one way, beside
`private_key_jwt`, for a machine client to authenticate and bind its tokens.

This server never terminates TLS itself. TLS ends at the infrastructure in
front of the two apps (DEPLOY.md), and the API sees plain http from the
apps' servers. It would only ever learn of a client certificate from a
header, which the proxy sets after checking the certificate.

## Decision

**mTLS is not supported.** A machine client authenticates with
`private_key_jwt` (ADR 0005), and binds its tokens with DPoP.

## Alternatives rejected

**Reading the certificate from a header set by a trusted proxy.** The header
would be a client credential that no signature protects. Its whole security
would rest on every deployment getting three things right:

- the proxy strips any copy of the header a client sends;
- only that proxy can reach the apps;
- each app passes the header on only from the proxy.

A single misconfiguration, such as a second ingress or a debugging port,
would let anyone authenticate as any client by writing a header. The apps
already pass on the caller's address from a header (`ADDRESS_HEADER`), but
that only decides a rate limit, never who a client is.

`private_key_jwt` with DPoP gives the same two properties without any of
that:

- **authentication** by a key only the client holds;
- **tokens bound** to a key, so a stolen token is useless without it.

Both are checked by this server itself, whatever stands in front of it.

## Consequences

- **What's lost:** a client that can only do mTLS, as some service meshes
  can, cannot use this server directly. It signs an assertion instead.
- **Discovery** does not list `tls_client_auth` or
  `tls_client_certificate_bound_access_tokens`.
- **Revisiting it:** if a deployment terminates TLS at the server, or a
  standard for forwarding certificates with a signature becomes usual, a
  new record can revisit this one.
