// Ports the unit test of tera-contrib/src/slug.rs: test_slug; its test_register is part of TestRegister.

package contrib

import "testing"

func TestSlug(t *testing.T) {
	checkFilter(t, Slug, "Hello World", nil, "hello-world")
	checkFilter(t, Slug, "Foo & Bar!", nil, "foo-bar")
}
