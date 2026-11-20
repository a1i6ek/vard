# 0007 Rotating `VARD_SECRET_KEY`

- **Status:** Accepted (2026-11-19)
- **Date:** 2026-11-19

## Context

Every secret the server keeps is sealed with a key derived from
`VARD_SECRET_KEY`:

- the signing keys and the installation's tag key;
- the provider secrets and the SMTP password;
- the two-step seeds;
- a replaced refresh token's successor, and a held SAML answer.

Each sealed value is AES-256-GCM, bound to its table, column and row, and
already starts with a format version and a 4-byte key id. Until now there was
one key, set once: a suspected leak could not be recovered from, because
under any other key nothing stored opened.

## Decision

**A keyring, and a command that moves every secret under the current key.**

- **`VARD_SECRET_KEY` stays the current key:** what is sealed from now on is
  sealed under it.
- **`VARD_OLD_SECRET_KEYS`** lists the keys before it, comma-separated. A
  value opens under whichever key its id names, still bound to its place.
- **`vard keys reseal`:**
  - goes through every sealed column (`store.SealedColumns`) in batches, in
    key order;
  - seals each value not under the current key again;
  - writes each back only if it is still the value read.

  It can be stopped and run again, and a second run changes nothing.

- **To rotate:**
  1. set a new `VARD_SECRET_KEY`, moving the old one into
     `VARD_OLD_SECRET_KEYS`;
  2. restart;
  3. run `keys reseal`;
  4. drop the old key and restart again.
- **The binding is unchanged.** Sealing is still bound to table, column and
  row, and `TestEverySealedColumnIsResealed` holds each column's binding to
  the model's own.
- **Strength rules:** an old key is not held to the strength rules. A weak
  key is exactly what an installation moves away from, so it must still
  open what it sealed.

## Alternatives rejected

- **`VARD_SECRET_KEYS`, one list, the current key first** (the plan's
  spelling). It changes the variable every deployment, script and compose
  file already sets. And in a list where order decides which key seals,
  getting the order wrong seals new secrets under a key being retired. A
  current key and a list of old ones can't be confused that way.
- **An outside KMS as the key-encryption key, now.** Envelope encryption
  through AWS KMS, Google Cloud KMS or Vault would put the root key outside
  the server's environment. It is left for a later record, because it brings:
  - a network dependency, and an availability one, to every start;
  - one client library per provider;
  - a configuration to get right on every installation.

  The keyring above is what a KMS would plug into: its unwrapped data key
  becomes `VARD_SECRET_KEY`.

- **Re-sealing on read, lazily.** Values nobody reads would stay under the
  old key forever, so the old key could never be dropped.

## Consequences

- **A leaked key can be rotated out.** Everything sealed under it is
  re-sealed, and the leaked key is then dropped.
- **Keyed hashes are untouched by a rotation.** Recovery codes, device user
  codes and throttle keys use the installation's own tag key, which is
  itself sealed and re-sealed. So none of them change.
- **A sealed column added later** goes into `store.SealedColumns`. The test
  fails until it does.
