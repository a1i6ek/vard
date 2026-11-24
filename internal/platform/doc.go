// Package platform is the shared kernel: the primitives every layer may
// use, knowing nothing of the server itself. crypto is the AEAD, hashing
// and randomness the secrets are sealed with; brand is the server's own
// name and mark; config is what the environment says. They sit below the
// domain, and nothing here may reach upward.
package platform
