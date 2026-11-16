package contrib

import (
	"github.com/Keats/tera/tera"
	"github.com/Keats/tera/tera-contrib/internal/slugify"
)

// Slug turns the given string into a URL slug: lowercase ASCII words joined by dashes.
//
//	{{ title | slug }}
var Slug = tera.TypedFilter(tera.ArgStr, func(val string, _ tera.Kwargs, _ *tera.State) (tera.Value, error) {
	return tera.NormalString(slugify.Slugify(val)), nil
})
