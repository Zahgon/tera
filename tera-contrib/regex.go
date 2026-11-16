package contrib

import (
	"sync"

	"github.com/Keats/tera/tera"
	"github.com/Keats/tera/tera-contrib/internal/regex"
)

// Striptags removes HTML tags and comments from its input. It does not guarantee well-formed output for input
// that is not valid HTML; in an autoescaped template the result needs the safe filter.
//
//	{{ value | striptags }}
var Striptags = tera.TypedFilter(tera.ArgStr, func(val string, _ tera.Kwargs, _ *tera.State) (tera.Value, error) {
	return tera.NormalString(regex.StripTags(val)), nil
})

// Spaceless removes whitespace between HTML tags. The result keeps the safety of its input.
//
//	{{ value | spaceless }}
var Spaceless = tera.TypedFilter(tera.ArgStringInput, func(val tera.StringInput, _ tera.Kwargs, _ *tera.State) (tera.Value, error) {
	return val.InheritSafety(regex.Spaceless(val.AsStr())), nil
})

// regexCache compiles each pattern once.
type regexCache struct {
	mu       sync.RWMutex
	compiled map[string]*regex.Regex
}

func (c *regexCache) get(pattern string) (*regex.Regex, error) {
	c.mu.RLock()
	r, ok := c.compiled[pattern]
	c.mu.RUnlock()
	if ok {
		return r, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	r, err := regex.Compile(pattern)
	if err != nil {
		return nil, tera.ErrorMessage("Invalid regex: " + err.Error())
	}
	c.compiled[pattern] = r
	return r, nil
}

// Matching is the `matching` test: true when the string value matches the regex in the `pat` argument. Each
// pattern is compiled once.
//
//	{% if value is matching(pat="^hello") %}...{% endif %}
type Matching struct{ cache regexCache }

// NewMatching returns a Matching test with an empty pattern cache.
func NewMatching() *Matching {
	return &Matching{cache: regexCache{compiled: map[string]*regex.Regex{}}}
}

// Call implements tera.Test.
func (m *Matching) Call(v tera.Value, kwargs tera.Kwargs, _ *tera.State) (bool, error) {
	val, err := tera.ArgStr(v)
	if err != nil {
		return false, err
	}
	pat, err := tera.KwargsMustGet(kwargs, "pat", tera.ArgStr)
	if err != nil {
		return false, err
	}
	r, err := m.cache.get(pat)
	if err != nil {
		return false, err
	}
	return r.IsMatch(val), nil
}

// RegexReplace is the `regex_replace` filter: it replaces every match of the `pattern` argument with `rep`,
// which may refer to capture groups as $1, $name or ${name}. Each pattern is compiled once.
//
//	{{ value | regex_replace(pattern="\d+", rep="") }}
type RegexReplace struct{ cache regexCache }

// NewRegexReplace returns a RegexReplace filter with an empty pattern cache.
func NewRegexReplace() *RegexReplace {
	return &RegexReplace{cache: regexCache{compiled: map[string]*regex.Regex{}}}
}

// Call implements tera.Filter.
func (f *RegexReplace) Call(v tera.Value, kwargs tera.Kwargs, _ *tera.State) (tera.Value, error) {
	val, err := tera.ArgStr(v)
	if err != nil {
		return tera.Undefined(), err
	}
	pattern, err := tera.KwargsMustGet(kwargs, "pattern", tera.ArgStr)
	if err != nil {
		return tera.Undefined(), err
	}
	rep, err := tera.KwargsMustGet(kwargs, "rep", tera.ArgStr)
	if err != nil {
		return tera.Undefined(), err
	}
	r, err := f.cache.get(pattern)
	if err != nil {
		return tera.Undefined(), err
	}
	return tera.NormalString(r.ReplaceAll(val, rep)), nil
}
