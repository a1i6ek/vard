# Architecture decision records

One file per significant decision, numbered in the order they were taken:
`NNNN-short-name.md`. A record is never rewritten once accepted; a later
decision that changes it is a new record that says which one it supersedes,
and the old one's status says "Superseded by NNNN".

Each record has four parts:

- **Context**: what forces the decision; the facts, not the answer.
- **Decision**: what was decided, in the active voice.
- **Alternatives rejected**: each with the reason.
- **Consequences**: what becomes easier, what becomes harder, what is now owed.

| Record                                                                                     | Status   |
| ------------------------------------------------------------------------------------------ | -------- |
| [0001 Scope](0001-scope.md)                                                                | Proposed |
| [0002 `cmd/` and `internal/`](0002-cmd-and-internal.md)                                    | Accepted |
| [0003 Split `internal/oidc`](0003-split-oidc.md)                                           | Accepted |
| [0004 Keep `store` and `model` whole](0004-store-and-model.md)                             | Accepted |
| [0005 Client profiles](0005-client-profiles.md)                                            | Accepted |
| [0006 No mutual TLS client authentication](0006-no-mtls.md)                                | Accepted |
| [0007 Rotating `VARD_SECRET_KEY`](0007-secret-key-versioning.md)                           | Accepted |
| [0008 Durable back-channel logout, and none on expiry](0008-durable-backchannel-logout.md) | Accepted |
| [0009 Depending on `crewjam/saml`, and the way out](0009-crewjam-saml.md)                  | Accepted |
| [0010 The passkey RP ID, chosen once](0010-passkey-rp-id.md)                               | Accepted |
| [0011 The HTTP API's description, and its versions](0011-api-versioning.md)                | Accepted |
