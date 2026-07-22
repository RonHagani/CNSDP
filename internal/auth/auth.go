// Package auth provides the shared bearer-token verification required at
// every product-exposed access path (NFR-012, AC-024). It exists so that
// every endpoint -- internal/intake, internal/retrieval, and any future
// one -- enforces identical parsing and comparison behavior, rather than
// each maintaining its own copy of security-sensitive logic that could
// drift out of sync.
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"
)

// Bearer reports whether header carries the exact configured token as an
// RFC 6750 "Bearer <token>" Authorization header value. Both the supplied
// and configured token are hashed to a fixed-length SHA-256 digest before
// subtle.ConstantTimeCompare -- comparing variable-length slices directly
// would return early on a length mismatch, leaking timing information
// about how close a guess's length is (PC-P-008: the platform protects
// itself).
func Bearer(header, token string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	supplied := sha256.Sum256([]byte(strings.TrimPrefix(header, prefix)))
	want := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(supplied[:], want[:]) == 1
}
