// Ports the unit tests of tera-contrib/src/json.rs: test_json_encode and test_json_encode_pretty; its test_register
// is part of TestRegister.

package contrib

import "testing"

func TestJSONEncode(t *testing.T) { checkFilter(t, JSONEncode, map[string]int{"a": 1}, nil, `{"a":1}`) }
func TestJSONEncodePretty(t *testing.T) {
	checkFilter(t, JSONEncode, map[string]int{"a": 1}, map[string]any{"pretty": true}, "{\n  \"a\": 1\n}")
}
