package authtoken

import (
	"encoding/hex"
	"testing"
)

func TestHashGolden(t *testing.T) {
	h := Hash("lineview-test-token")
	got := hex.EncodeToString(h[:])
	const want = "8dee72ab26fdd1f5b18492534d82af35d010f0a3a91728f4fd047fcfae34f967"
	if got != want {
		t.Fatalf("Hash changed:\n got %s\n want %s", got, want)
	}
}

func TestHashIsDeterministicAndInputSensitive(t *testing.T) {
	if Hash("a") != Hash("a") {
		t.Fatal("same input gave different hashes")
	}
	if Hash("a") == Hash("b") {
		t.Fatal("different inputs gave the same hash")
	}
}

// Pins the contract: hashing the hext TEXT differs from hashing the decoded
// bytes. Both sides must use the text.
func TestHashesTextNotDecodedBytes(t *testing.T) {
	text := "00ff"
	raw, _ := hex.DecodeString(text)
	if Hash(text) == Hash(string(raw)) {
		t.Fatal("text and decoded-bytes hashes unexpectedly equal")
	}
}
