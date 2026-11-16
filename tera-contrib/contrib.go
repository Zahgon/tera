// Package contrib holds the additional Tera filters, tests and functions that rely on third-party behaviour:
// base64, dates, file sizes, Rust-style formatting, JSON, seeded randomness, regular expressions, slugs and URL
// encoding. Each one is registered on a Tera instance under the name it is documented with, either one by one
// (for example t.RegisterFilter("b64_encode", contrib.B64Encode)) or all at once with Register.
package contrib

import "github.com/Keats/tera/tera"

// Register adds every filter, test and function of this package to t under its documented name.
func Register(t *tera.Tera) {
	t.RegisterFilter("b64_encode", B64Encode)
	t.RegisterFilter("b64_decode", B64Decode)
	t.RegisterFunction("now", Now)
	t.RegisterFilter("date", Date)
	t.RegisterTest("before", IsBefore)
	t.RegisterTest("after", IsAfter)
	t.RegisterFilter("filesize_format", FilesizeFormat)
	t.RegisterFilter("format", Format)
	t.RegisterFilter("json_encode", JSONEncode)
	t.RegisterFunction("get_random", GetRandom)
	t.RegisterFilter("shuffle", Shuffle)
	t.RegisterFilter("striptags", Striptags)
	t.RegisterFilter("spaceless", Spaceless)
	t.RegisterFilter("regex_replace", NewRegexReplace())
	t.RegisterTest("matching", NewMatching())
	t.RegisterFilter("slug", Slug)
	t.RegisterFilter("urlencode", URLEncode)
	t.RegisterFilter("urlencode_strict", URLEncodeStrict)
}
