package contrib

import "testing"

func TestSlug(t *testing.T) {
	checkFilter(t, Slug, "Hello World", nil, "hello-world")
	checkFilter(t, Slug, "Foo & Bar!", nil, "foo-bar")
}
