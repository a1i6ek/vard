# 0009 Depending on `crewjam/saml`, and the way out

- **Status:** Accepted (2026-11-21)
- **Date:** 2026-11-21

## Context

SAML sign-in rests on `github.com/crewjam/saml` (v0.5.1) for parsing and
verifying responses, with `russellhaering/goxmldsig` for XML signatures. XML
signature handling is where SAML libraries fail: signature wrapping, comments
in names, namespaces read loosely. The library's releases are infrequent, and
one advisory against it (GO-2026-6909) has no fixed release at all. If it
stopped being maintained, a vulnerability found later might never be fixed
upstream.

## Decision

**Keep `crewjam/saml`, behind checks of our own, and watch it in CI.**

- **Before the library parses**, `checkSAMLAssertions` and
  `checkEncryptedAlgorithms` (`internal/federation/sso_saml.go`) refuse, on
  the raw XML, encrypted assertions included:
  - an assertion without its SAML `Issuer`;
  - a `NameID` outside SAML's namespace (GO-2026-6909);
  - an assertion not signed itself;
  - SHA-1 signatures and RSA-1_5 key transport.
- **Around it:**
  - only the HTTP-POST binding, so the library never fetches an artifact;
  - a bearer confirmation;
  - each assertion ID used once;
  - a NameID taken as an address only when its format says so.
- **In CI:** `make vuln` (govulncheck) runs on every push. An advisory the
  code reaches fails the build unless the Makefile's `VULN_IGNORE` names it,
  with the reason and the check that answers it.
- **The library is used in two files only:** `internal/federation/sso_saml.go`
  and the SSO API's response. Replacing it touches nothing else.

## The way out

If the library goes unmaintained (no release for a year while an advisory
stands, or the repository archived):

1. **First:** fork it into the module (`internal/third_party/saml`). Apply the
   fixes there, keep its tests, and keep the checks above.
2. **If that grows:** move to another maintained Go SAML service-provider
   library, or write the service provider's narrow part ourselves (POST
   binding, response and assertion verification through `goxmldsig`). A
   service provider needs far less than the library offers.

## Alternatives rejected

- **Write SAML ourselves now.** Hand-written XML signature verification is
  how most SAML vulnerabilities start. The library, with checks in front of
  it, is the smaller risk today.
- **Drop SAML.** Enterprises still need it; OIDC doesn't replace it for them.
- **Ignore advisories that need the provider's signature.** A provider's key
  can leak, and the checks cost little.

## Consequences

- **Watched:** a new advisory fails CI the day it is published, not at the
  next local `make ci`.
- **Maintained by us:** each check is ours to keep until a release makes it
  unneeded. The Makefile says when to drop each ignore.
- **Signed assertions required:** a provider that signs only the response,
  not the assertion, is refused. The metadata has always asked for signed
  assertions (`WantAssertionsSigned`).
