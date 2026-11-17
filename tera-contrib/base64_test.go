// Ports the unit tests of tera-contrib/src/base64.rs: test_b64_encode, test_b64_encode_no_padding,
// test_b64_encode_url_safe, test_b64_decode, test_b64_decode_url_safe, test_b64_roundtrip and
// test_b64_decode_invalid; its test_register is part of TestRegister in contrib_test.go.

package contrib

import (
	"github.com/Keats/tera/tera"
	"testing"
)

func TestB64Encode(t *testing.T) { checkFilter(t, B64Encode, "hello world", nil, "aGVsbG8gd29ybGQ=") }
func TestB64EncodeNoPadding(t *testing.T) {
	checkFilter(t, B64Encode, "hello world", map[string]any{"padded": false}, "aGVsbG8gd29ybGQ")
}
func TestB64EncodeURLSafe(t *testing.T) {
	checkFilter(t, B64Encode, "<<??>>", map[string]any{"url_safe": true}, "PDw_Pz4-")
}
func TestB64Decode(t *testing.T) {
	for _, s := range []string{"aGVsbG8gd29ybGQ=", "aGVsbG8gd29ybGQ"} {
		checkFilter(t, B64Decode, s, nil, "hello world")
	}
}
func TestB64DecodeURLSafe(t *testing.T) {
	checkFilter(t, B64Decode, "PDw_Pz4-", map[string]any{"url_safe": true}, "<<??>>")
}
func TestB64Roundtrip(t *testing.T) {
	original := "Hello, 世界! 🌍"
	state := tera.NewState(tera.NewContext())
	encoded, err := B64Encode.Call(value(t, original), tera.Kwargs{}, state)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := B64Decode.Call(encoded, tera.Kwargs{}, state)
	if err != nil || decoded.String() != original {
		t.Fatalf("expected %q, got %q, %v", original, decoded.String(), err)
	}
}
func TestB64DecodeInvalid(t *testing.T) { checkFilterError(t, B64Decode, "not valid base64!!!", nil) }
