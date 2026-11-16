package contrib

import (
	"github.com/Keats/tera/tera"
	"testing"
)

func TestStriptags(t *testing.T) {
	for _, tc := range [][2]string{
		{`<b>Joel</b> <button>is</button> a <span>slug</span>`, "Joel is a slug"},
		{`<p>just a small   \n <a href="x"> example</a> link</p>\n<p>to a webpage</p><!-- <p>and some commented stuff</p> -->`, `just a small   \n  example link\nto a webpage`},
		{`<p>See: &#39;&eacute; is an apostrophe followed by e acute</p>`, `See: &#39;&eacute; is an apostrophe followed by e acute`},
		{"<adf>a", "a"}, {"</adf>a", "a"}, {"<asdf><asdf>e", "e"}, {"hi, <f x", "hi, <f x"}, {"234<235, right?", "234<235, right?"}, {"a4<a5 right?", "a4<a5 right?"}, {"b7>b2!", "b7>b2!"}, {"</fe", "</fe"}, {"<x>b<y>", "b"}, {"a<p a >b</p>c", "abc"}, {"d<a:b c:d>e</p>f", "def"}, {`<strong>foo</strong><a href="http://example.com">bar</a>`, "foobar"}, {"a &amp; b", "a &amp; b"},
	} {
		checkFilter(t, Striptags, tc[0], nil, tc[1])
	}
}
func TestSpaceless(t *testing.T) {
	for _, tc := range [][2]string{{"<p>\n<a>test</a>\r\n </p>", "<p><a>test</a></p>"}, {"<p>\n<a> </a>\r\n </p>", "<p><a></a></p>"}, {"<p> </p>", "<p></p>"}, {"<p> <a>", "<p><a>"}, {"<p> test</p>", "<p> test</p>"}, {"<p>\r\n</p>", "<p></p>"}} {
		checkFilter(t, Spaceless, tc[0], nil, tc[1])
	}
}
func TestMatching(t *testing.T) {
	for _, tc := range []struct {
		input, pattern string
		want           bool
	}{{"abc", "b", true}, {"abc", "^b$", false}, {"Hello, World!", `(?i)(hello\W\sworld\W)`, true}, {"The date was 2018-06-28", `\d{4}-\d{2}-\d{2}$`, true}} {
		got, err := NewMatching().Call(value(t, tc.input), kwargs(t, map[string]any{"pat": tc.pattern}), tera.NewState(tera.NewContext()))
		if err != nil || got != tc.want {
			t.Errorf("expected %t, got %t, %v", tc.want, got, err)
		}
	}
}
func TestRegexReplace(t *testing.T) {
	filter := NewRegexReplace()
	for _, tc := range [][4]string{{"Springsteen, Bruce", `(?P<last>[^,\s]+),\s+(?P<first>\S+)`, "$first $last", "Bruce Springsteen"}, {"abc123def456", `\d+`, "X", "abcXdefX"}, {"hello world", "zzz", "X", "hello world"}} {
		checkFilter(t, filter, tc[0], map[string]any{"pattern": tc[1], "rep": tc[2]}, tc[3])
	}
}
func TestRegexReplaceInvalidPattern(t *testing.T) {
	checkFilterError(t, NewRegexReplace(), "test", map[string]any{"pattern": "[invalid", "rep": "X"})
}
