package contrib

import "testing"

func TestURLEncode(t *testing.T) {
	checkFilter(t, URLEncode, "hello world", nil, "hello%20world")
	checkFilter(t, URLEncode, "foo/bar?baz=1", nil, "foo/bar%3Fbaz%3D1")
}
func TestURLEncodeStrict(t *testing.T) {
	checkFilter(t, URLEncodeStrict, "hello", nil, "hello")
	checkFilter(t, URLEncodeStrict, "a/b", nil, "a%2Fb")
}
