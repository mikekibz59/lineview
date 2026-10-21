package authtoken

import "crypto/sha256"

// Hash returns the stored identity of a session token. It is pure and
// deterministic. The length is fixed by the type, mirroring the table's
// CHECK (octet_length(token_hash) = 32)
func Hash(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}
