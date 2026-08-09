// Package auth contains process-local Basic Auth primitives shared by Atlassian modules.
package auth

import "fmt"

// Credential holds one Basic Auth username/password pair in memory.
type Credential struct {
	username string
	password string
}

// NewCredential returns an immutable credential value. Passwords are exposed only through
// Password so callers can apply them directly to outbound Basic Auth requests.
func NewCredential(username, password string) Credential {
	return Credential{username: username, password: password}
}

// Username returns the Basic Auth username.
func (c Credential) Username() string { return c.username }

// Password returns the Basic Auth password for immediate request use.
func (c Credential) Password() string { return c.password }

// String redacts the password so accidental formatting does not leak secrets.
func (c Credential) String() string {
	return fmt.Sprintf("Credential{username:%q,password:%s}", c.username, "[REDACTED]")
}
