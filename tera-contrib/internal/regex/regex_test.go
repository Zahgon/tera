// Reference vectors recorded from the original crate at its locked version (a sample of the full differential set).
package regex

import (
	"strings"
	"testing"
)

func TestRustVectors(t *testing.T) {
	cases := []struct {
		pattern, rep, subject string
		match                 bool
		replaced, err         string
	}{
		{"[", "X", "", false, "", "regex parse error:\n    [\n    ^\nerror: unclosed character class"},
		{"[abc", "$$$0/${a}/${unknown}/$99999", "", false, "", "regex parse error:\n    [abc\n    ^\nerror: unclosed character class"},
		{"[]", "$$$0/${a}/${unknown}/$99999", "", false, "", "regex parse error:\n    []\n    ^^\nerror: unclosed character class"},
		{"[^]", "X", "", false, "", "regex parse error:\n    [^]\n    ^^^\nerror: unclosed character class"},
		{"[z-a]", "", "", false, "", "regex parse error:\n    [z-a]\n     ^^^\nerror: invalid character class range, the start must be <= the end"},
		{"[\\d-a]", "$0|$1|$a|$$|$1x|${1}x", "", false, "", "regex parse error:\n    [\\d-a]\n     ^^\nerror: invalid range boundary, must be a literal"},
		{"[a-\\d]", "${}/$é/${é}/$0x/$01/${01}/$", "", false, "", "regex parse error:\n    [a-\\d]\n       ^^\nerror: invalid range boundary, must be a literal"},
		{"\\", "X", "", false, "", "regex parse error:\n    \\\n    ^\nerror: incomplete escape sequence, reached end of pattern prematurely"},
		{"\\q", "", "", false, "", "regex parse error:\n    \\q\n    ^^\nerror: unrecognized escape sequence"},
		{"\\1", "${}/$é/${é}/$0x/$01/${01}/$", "", false, "", "regex parse error:\n    \\1\n    ^^\nerror: backreferences are not supported"},
		{"\\0", "$0|$1|$a|$$|$1x|${1}x", "", false, "", "regex parse error:\n    \\0\n    ^^\nerror: backreferences are not supported"},
		{"\\9", "$$$0/${a}/${unknown}/$99999", "", false, "", "regex parse error:\n    \\9\n    ^^\nerror: backreferences are not supported"},
		{"\\x", "X", "", false, "", "regex parse error:\n    \\x\n      ^\nerror: incomplete escape sequence, reached end of pattern prematurely"},
		{"\\xG0", "X", "", false, "", "regex parse error:\n    \\xG0\n      ^\nerror: invalid hexadecimal digit"},
		{"\\x0G", "$$$0/${a}/${unknown}/$99999", "", false, "", "regex parse error:\n    \\x0G\n       ^\nerror: invalid hexadecimal digit"},
		{"\\x{", "${}/$é/${é}/$0x/$01/${01}/$", "", false, "", "regex parse error:\n    \\x{\n      ^\nerror: incomplete escape sequence, reached end of pattern prematurely"},
		{"\\x{}", "X", "", false, "", "regex parse error:\n    \\x{}\n      ^^\nerror: hexadecimal literal empty"},
		{"\\x{110000}", "$$$0/${a}/${unknown}/$99999", "", false, "", "regex parse error:\n    \\x{110000}\n       ^^^^^^\nerror: hexadecimal literal is not a Unicode scalar value"},
		{"\\u123", "X", "", false, "", "regex parse error:\n    \\u123\n         ^\nerror: incomplete escape sequence, reached end of pattern prematurely"},
		{"\\U0000ffff", "X", "", false, "", ""},
		{"\\U0000ffff", "X", "abc", false, "abc", ""},
		{"\\U0000ffff", "X", "é中", false, "é中", ""},
		{"\\U0000ffff", "X", "١۲３", false, "١۲３", ""},
		{"\\U0000ffff", "X", "aé b", false, "aé b", ""},
		{"\\U0000ffff", "X", "\n", false, "\n", ""},
		{"\\U0000ffff", "X", "Kkſs", false, "Kkſs", ""},
		{"\\U0000ffff", "X", " ", false, " ", ""},
		{"\\U0000ffff", "X", "a-b_c", false, "a-b_c", ""},
		{"\\U0000ffff", "X", "a\r\nb\rc\n", false, "a\r\nb\rc\n", ""},
		{"\\U0000ffff", "X", "\u2028", false, "\u2028", ""},
		{"(?i-i)", "", "", false, "", "regex parse error:\n    (?i-i)\n      ^ ^\nerror: duplicate flag"},
		{"a**", "X", "a\r\nb\rc\n", true, "X\rX\nXbX\rXcX\nX", ""},
		{"a{2}*", "X", "a a", true, "XaX XaX", ""},
		{"\\b{start}", "X", "𝟘", true, "X𝟘", ""},
		{"\\a", "${}/$é/${é}/$0x/$01/${01}/$", "abc", false, "abc", ""},
		{"\\f", "$0|$1|$a|$$|$1x|${1}x", "‌", false, "‌", ""},
		{"a+", "$0|$1|$a|$$|$1x|${1}x", "ab aa aaa", true, "a|||$||xb aa|||$||x aaa|||$||x", ""},
		{"(?:\\p{XID_Start})*\\p{gc:Letter}", "${}/$é/${é}/$0x/$01/${01}/$", "Iıİi", true, "/$é/////$", ""},
		{"(?s)\\p{L}+", "", "\ufeff", false, "\ufeff", ""},
		{"(?-u)é{1,3}", "${}/$é/${é}/$0x/$01/${01}/$", "a\nb", false, "a\nb", ""},
		{"(?i)[a-z&&[^aeiou]]{1,3}", "$$$0/${a}/${unknown}/$99999", "ab aa aaa", true, "a$b/// aa aaa", ""},
		{"#*s*", "", "Iıİi", true, "Iıİi", ""},
		{"(?-u)[a-]+", "", "\ufeff", false, "\ufeff", ""},
		{"a0b", "${}/$é/${é}/$0x/$01/${01}/$", "A a", false, "A a", ""},
		{"(?U)[[:alpha:]]*?", "$$$0/${a}/${unknown}/$99999", "a-b_c", true, "$a///-$b///_$c///", ""},
		{"(?-u)^+", "", "Iıİi", true, "Iıİi", ""},
		{"(?U)[a-cx]*?", "$0|$1|$a|$$|$1x|${1}x", "\ufeff", true, "|||$||x\ufeff|||$||x", ""},
		{"(?m)(?P<a>\\w)|(?<b>\\p{Uppercase})", "", "a\nb", true, "\n", ""},
		{"(?-u)[--a]{1,3}", "$0|$1|$a|$$|$1x|${1}x", "hello 世界", false, "hello 世界", ""},
		{"ab}", "X", "Iıİi", false, "Iıİi", ""},
		{"(?m)\\p{Uppercase}+", "X", "", false, "", ""},
		{"(?U)[a&&]", "$$$0/${a}/${unknown}/$99999", "aé b", false, "aé b", ""},
		{"(?m)(?:[a-z])*[&&]", "$0|$1|$a|$$|$1x|${1}x", " ", false, " ", ""},
		{"(?s)\\p{ASCII}*?", "$0|$1|$a|$$|$1x|${1}x", "\u2028", true, "|||$||x\u2028|||$||x", ""},
		{"(?i)[-a]+", "X", "bbaab", true, "bbXb", ""},
		{"[-a]+", "${}/$é/${é}/$0x/$01/${01}/$", "aé b", true, "/$é/////$é b", ""},
		{"(?-u)\\b{start-half}{1,3}", "${}/$é/${é}/$0x/$01/${01}/$", "ab aa aaa", true, "/$é/////$ab /$é/////$aa /$é/////$aaa", ""},
		{"(?i)\\x41+", "", "Iıİi", false, "Iıİi", ""},
		{"(?i)\\b{start-half}", "${}/$é/${é}/$0x/$01/${01}/$", "\ufeff", true, "/$é/////$\ufeff/$é/////$", ""},
		{"\\d+", "$$$0/${a}/${unknown}/$99999", "\n", false, "\n", ""},
		{"(?s)\\b*?", "", "a-b_c", true, "a-b_c", ""},
		{"${1,3}", "X", "😀", true, "😀X", ""},
		{"(?i)(\\s)(\\p{Script=Latin})", "X", "", false, "", ""},
		{"(?-u)\\u0041*?", "$0|$1|$a|$$|$1x|${1}x", "\n", true, "|||$||x\n|||$||x", ""},
		{"\\w+", "", "ab aa aaa", true, "  ", ""},
		{"(?mR)\\b", "", "Iıİi", true, "Iıİi", ""},
		{"(?i)[]a]", "$0|$1|$a|$$|$1x|${1}x", "\ufeff", false, "\ufeff", ""},
		{"(?i)[a-z]{1,3}", "${}/$é/${é}/$0x/$01/${01}/$", "A a", true, "/$é/////$ /$é/////$", ""},
		{"(?m)\\b*?", "$$$0/${a}/${unknown}/$99999", "hello 世界", true, "$///h$///e$///l$///l$///o$/// $///世$///界$///", ""},
		{"}:*i}s\\++<]m", "$0|$1|$a|$$|$1x|${1}x", "Iıİi", false, "Iıİi", ""},
		{"(?m)\\p{Assigned}+", "${}/$é/${é}/$0x/$01/${01}/$", "𝟘", true, "/$é/////$", ""},
		{"(?i)(?P<a>\\u0041)|(?<b>\\p{Assigned})", "$0|$1|$a|$$|$1x|${1}x", "١۲３", true, "١|||$||x۲|||$||x３|||$||x", ""},
		{"(\\b{end-half})(é)", "${}/$é/${é}/$0x/$01/${01}/$", "> <", false, "> <", ""},
		{"(?i)[~~a]{1,3}", "X", "\u2028", false, "\u2028", ""},
		{"(?m)\\D+", "$$$0/${a}/${unknown}/$99999", "𝟘", false, "𝟘", ""},
		{"(?m)(?P<a>\\D)|(?<b>$)", "${}/$é/${é}/$0x/$01/${01}/$", "١۲３", true, "١۲３/$é/////$", ""},
		{"(?i)(\\p{Script=Latin})(\\p{Join_Control})", "", " ", false, " ", ""},
		{"(?s)[-a]", "", "\r", false, "\r", ""},
		{"a", "$0|$1|$a|$$|$1x|${1}x", "a a", true, "a|||$||x a|||$||x", ""},
		{"(?x)", "$$$0/${a}/${unknown}/$99999", "", true, "$///", ""},
		{"(?m)(?P<a>\\W)|(?<b>[[:alpha:]])", "", "‌", false, "‌", ""},
		{"(?s)\\D{1,3}", "${}/$é/${é}/$0x/$01/${01}/$", "\r", true, "/$é/////$", ""},
		{"(?s)[a-f--[b-d]]", "X", "aaaa", true, "XXXX", ""},
		{"(?i)\\B{1,3}", "", "١۲３", true, "١۲３", ""},
		{"(?i).", "", "> <", true, "", ""},
		{"(?U)[a-]*?", "", "áb", true, "́b", ""},
		{"(?s)\\p{gc:Letter}+", "${}/$é/${é}/$0x/$01/${01}/$", "𝟘", false, "𝟘", ""},
		{"^>bq", "", "\n", false, "\n", ""},
		{"(?-u)\\w+", "${}/$é/${é}/$0x/$01/${01}/$", "ab aa aaa", true, "/$é/////$ /$é/////$ /$é/////$", ""},
		{"(\\b)([[:alpha:]])", "$$$0/${a}/${unknown}/$99999", "Iıİi", true, "$I///ıİi", ""},
		{"(?P<a>\\p{ASCII})|(?<b>\\p{Uppercase})", "X", "", false, "", ""},
		{"\\D|[a-cx]", "$0|$1|$a|$$|$1x|${1}x", "a\nb", true, "a|||$||x\n|||$||xb|||$||x", ""},
		{"(?mR)\\p{ASCII}+", "${}/$é/${é}/$0x/$01/${01}/$", "a-b_c", true, "/$é/////$", ""},
		{"(?U)\\A*?", "${}/$é/${é}/$0x/$01/${01}/$", "Iıİi", true, "/$é/////$I/$é/////$ı/$é/////$İ/$é/////$i/$é/////$", ""},
		{"\\(m>bi?", "$0|$1|$a|$$|$1x|${1}x", "\ufeff", false, "\ufeff", ""},
		{"(?i)中|[^a]", "$$$0/${a}/${unknown}/$99999", "aé b", true, "a$é///$ ///$b///", ""},
		{"(?i)[-a]*?", "", "‌", true, "‌", ""},
		{"(\\b{end-half})(\\p{Alphabetic})", "X", "\r", false, "\r", ""},
		{"\\W{1,3}", "${}/$é/${é}/$0x/$01/${01}/$", ":", true, "/$é/////$", ""},
		{"(?x)[--a]*?", "", "", true, "", ""},
		{"(?-u)[--a]", "$$$0/${a}/${unknown}/$99999", "‌", false, "‌", ""},
		{"(?i)[[:^digit:]]+", "X", "aaab", true, "X", ""},
		{"q^.}", "X", ":", false, ":", ""},
		{"(?-u)[]a]", "$$$0/${a}/${unknown}/$99999", "", false, "", ""},
		{"(?U)[]a]", "X", "Kkſs", false, "Kkſs", ""},
		{"s.9", "$$$0/${a}/${unknown}/$99999", "a\r\nb\rc\n", false, "a\r\nb\rc\n", ""},
		{"(?-u)[a-z]+", "$0|$1|$a|$$|$1x|${1}x", "a a", true, "a|||$||x a|||$||x", ""},
		{"(?i)(?P<a>\\b)|(?<b>[[:^digit:]])", "$$$0/${a}/${unknown}/$99999", "é中", true, "$///é$中///", ""},
		{"(?m)([[:^digit:]])(^)", "$$$0/${a}/${unknown}/$99999", "́", false, "́", ""},
		{"|9a2m9}^|", "$0|$1|$a|$$|$1x|${1}x", "a\r\nb\rc\n", true, "|||$||xa|||$||x\r|||$||x\n|||$||xb|||$||x\r|||$||xc|||$||x\n|||$||x", ""},
		{"(?m)(?:\\S)*é", "$0|$1|$a|$$|$1x|${1}x", "a a", false, "a a", ""},
		{"(?U)\\u0041*?", "$$$0/${a}/${unknown}/$99999", "é中", true, "$///é$///中$///", ""},
		{"(?U)[-a]+", "", "́", false, "́", ""},
		{"(?i)\\p{Any}", "$$$0/${a}/${unknown}/$99999", "\r\n", true, "$\r///$\n///", ""},
		{"\\<*?", "X", "aaaa", true, "XaXaXaXaX", ""},
		{"(?:[--a])*\\p{Lowercase}", "$$$0/${a}/${unknown}/$99999", "é中", true, "$é///中", ""},
		{"(?m)\\>{1,3}", "", " ", false, " ", ""},
		{"(?i)(?:[--a])*\\>", "${}/$é/${é}/$0x/$01/${01}/$", "\r", false, "\r", ""},
		{"[>0**\\#^9b<]}", "$0|$1|$a|$$|$1x|${1}x", "a a", false, "a a", ""},
		{"\\p{Join_Control}*?", "$$$0/${a}/${unknown}/$99999", "é中", true, "$///é$///中$///", ""},
		{"^>-.a*qq1*|a9", "", "́", false, "́", ""},
		{"(?m)\\b+", "", "a\r\nb\rc\n", true, "a\r\nb\rc\n", ""},
		{"(?U)[--a]+", "$0|$1|$a|$$|$1x|${1}x", "a a", true, "a|||$||x a|||$||x", ""},
		{"(?i)\\B*?", "${}/$é/${é}/$0x/$01/${01}/$", "abc", true, "/$é/////$a/$é/////$b/$é/////$c/$é/////$", ""},
		{"0.", "X", "‌", false, "‌", ""},
		{"#-*b>:b[<]]:1", "${}/$é/${é}/$0x/$01/${01}/$", "aaab", false, "aaab", ""},
		{"(?m)(?P<a>[a-z&&[^aeiou]])|(?<b>\\b{start-half})", "", "zz", true, "", ""},
		{"(?-us-)", "", "", false, "", "regex parse error:\n    (?-us-)\n      ^  ^\nerror: flag negation operator repeated"},
		{".0}+}-am<", "${}/$é/${é}/$0x/$01/${01}/$", "A a", false, "A a", ""},
		{"(?i)[a-z&&[^aeiou]]+", "X", "hello 世界", true, "XeXo 世界", ""},
		{"a|\\p{gc!=Lu}", "X", "\u2028", true, "X", ""},
		{"(?m)é*?", "${}/$é/${é}/$0x/$01/${01}/$", "𝟘", true, "/$é/////$𝟘/$é/////$", ""},
		{"(?m)(?P<a>[a-])|(?<b>\\p{Assigned})", "$0|$1|$a|$$|$1x|${1}x", "aé b", true, "a|a|a|$||axé|||$||x |||$||xb|||$||x", ""},
		{"(?i)(?P<a>\\p{Join_Control})|(?<b>\\p{gc!=Lu})", "", "ab aa aaa", true, "abaaaaa", ""},
		{"\\p{XID_Start}", "X", "", false, "", ""},
	}
	compiled := map[string]*Regex{}
	for i, c := range cases {
		r, ok := compiled[c.pattern]
		if !ok {
			var err error
			r, err = Compile(c.pattern)
			message := ""
			if err != nil {
				message = err.Error()
			}
			if message != c.err {
				t.Errorf("case %d compile %q: %q != %q", i, c.pattern, message, c.err)
			}
			if err != nil {
				continue
			}
			compiled[c.pattern] = r
		}
		if got := r.IsMatch(c.subject); got != c.match {
			t.Errorf("case %d %q match %q: %v != %v", i, c.pattern, c.subject, got, c.match)
		}
		if got := r.ReplaceAll(c.subject, c.rep); got != c.replaced {
			t.Errorf("case %d %q replace %q: %q != %q", i, c.pattern, c.subject, got, c.replaced)
		}
	}
}

func TestHTMLFilters(t *testing.T) {
	if got, want := StripTags("<p>Hi</p><!-- bye -->"), "Hi"; got != want {
		t.Fatalf("StripTags = %q, want %q", got, want)
	}
	if got, want := Spaceless("<p>\u2003<em>x</em>\n</p>"), "<p><em>x</em></p>"; got != want {
		t.Fatalf("Spaceless = %q, want %q", got, want)
	}
}

// TestRustNestingUnicodeNamesAndLargeClasses checks three limits of the regex crate the original compiles
// patterns with: groups, classes and repetitions nested past the parser's limit of 250 levels are rejected,
// capture names follow the Rust toolchain's Unicode letter and number tables, and repeated Unicode classes hit
// the compiled size limit. Expected values were recorded from regex 1.13.1, the version the original locks.
func TestRustNestingUnicodeNamesAndLargeClasses(t *testing.T) {
	nested := func(open, core, close string, depth int) string {
		return strings.Repeat(open, depth) + core + strings.Repeat(close, depth)
	}
	// nestingError is the parse error for a pattern nested too deeply, its carets at the recorded column.
	nestingError := func(pattern string, column, width int) string {
		return "regex parse error:\n    " + pattern + "\n    " + strings.Repeat(" ", column) +
			strings.Repeat("^", width) + "\nerror: exceed the maximum number of nested parentheses/brackets (250)"
	}
	const sizeError = "Compiled regex exceeds size limit of 10485760 bytes."
	subjects := []string{"a", "éééééééééé", "", "ab", "ñ名ab"}
	const rep = "$0/${\u0345}/${a\u00b2}/${\U0001e6c0}/${名前}/${ñ}"
	cases := []struct {
		pattern string
		match   []bool
		want    []string
		err     string
	}{
		{pattern: nested("(", "ab", ")", 249), match: []bool{false, false, false, true, true}, want: []string{"a", "éééééééééé", "", "ab/////", "ñ名ab/////"}},
		{pattern: nested("(?:", "a", ")", 249), match: []bool{true, false, false, true, true}, want: []string{"a/////", "éééééééééé", "", "a/////b", "ñ名a/////b"}},
		{pattern: nested("[", "ab", "]", 249), match: []bool{true, false, false, true, true}, want: []string{"a/////", "éééééééééé", "", "a/////b/////", "ñ名a/////b/////"}},
		{pattern: nested("(", "ab", ")", 250), err: nestingError(nested("(", "ab", ")", 250), 250, 2)},
		{pattern: nested("(?:", "a", ")", 250), match: []bool{true, false, false, true, true}, want: []string{"a/////", "éééééééééé", "", "a/////b", "ñ名a/////b"}},
		{pattern: nested("[", "ab", "]", 250), err: nestingError(nested("[", "ab", "]", 250), 250, 2)},
		{pattern: nested("(", "ab", ")", 251), err: nestingError(nested("(", "ab", ")", 251), 250, 4)},
		{pattern: nested("(?:", "a", ")", 251), err: nestingError(nested("(?:", "a", ")", 251), 750, 5)},
		{pattern: nested("[", "ab", "]", 251), err: nestingError(nested("[", "ab", "]", 251), 250, 4)},
		{pattern: "a" + strings.Repeat("*", 249), match: []bool{true, true, true, true, true}, want: []string{"a/////", "/////é/////é/////é/////é/////é/////é/////é/////é/////é/////é/////", "/////", "a/////b/////", "/////ñ/////名a/////b/////"}},
		{pattern: "a" + strings.Repeat("*", 250), match: []bool{true, true, true, true, true}, want: []string{"a/////", "/////é/////é/////é/////é/////é/////é/////é/////é/////é/////é/////", "/////", "a/////b/////", "/////ñ/////名a/////b/////"}},
		{pattern: "a" + strings.Repeat("*", 251), err: nestingError("a"+strings.Repeat("*", 251), 0, 2)},
		{pattern: "(?P<\u0345>a)", match: []bool{true, false, false, true, true}, want: []string{"a/a////", "éééééééééé", "", "a/a////b", "ñ名a/a////b"}},
		{pattern: "(?P<a\u00b2>a)", match: []bool{true, false, false, true, true}, want: []string{"a//a///", "éééééééééé", "", "a//a///b", "ñ名a//a///b"}},
		{pattern: "(?P<\U0001e6c0>a)", match: []bool{true, false, false, true, true}, want: []string{"a///a//", "éééééééééé", "", "a///a//b", "ñ名a///a//b"}},
		{pattern: "(?P<名前>a)", match: []bool{true, false, false, true, true}, want: []string{"a////a/", "éééééééééé", "", "a////a/b", "ñ名a////a/b"}},
		{pattern: "(?P<ñ>a)", match: []bool{true, false, false, true, true}, want: []string{"a/////a", "éééééééééé", "", "a/////ab", "ñ名a/////ab"}},
		{pattern: "(?P<_x>a)", match: []bool{true, false, false, true, true}, want: []string{"a/////", "éééééééééé", "", "a/////b", "ñ名a/////b"}},
		{pattern: "(?P<a.b[0]>a)", match: []bool{true, false, false, true, true}, want: []string{"a/////", "éééééééééé", "", "a/////b", "ñ名a/////b"}},
		{pattern: "(?P<1a>a)", err: "regex parse error:\n    (?P<1a>a)\n        ^\nerror: invalid capture group character"},
		{pattern: "(?P<a-b>a)", err: "regex parse error:\n    (?P<a-b>a)\n         ^\nerror: invalid capture group character"},
		{pattern: "(?P<>a)", err: "regex parse error:\n    (?P<>a)\n        ^\nerror: empty capture group name"},
		{pattern: "(?P<é\u0301>a)", err: "regex parse error:\n    (?P<é\u0301>a)\n         ^\nerror: invalid capture group character"},
		{pattern: "(?P<n>a)(?P<n>b)", err: "regex parse error:\n    (?P<n>a)(?P<n>b)\n        ^       ^\nerror: duplicate capture group name"},
		{pattern: "(?<ñ>a)", match: []bool{true, false, false, true, true}, want: []string{"a/////a", "éééééééééé", "", "a/////ab", "ñ名a/////ab"}},
	}
	for _, c := range cases {
		r, err := Compile(c.pattern)
		message := ""
		if err != nil {
			message = err.Error()
		}
		if message != c.err {
			t.Errorf("Compile(%.40q) error %q, want %q", c.pattern, message, c.err)
		}
		if err != nil {
			continue
		}
		for i, subject := range subjects {
			if got := r.IsMatch(subject); got != c.match[i] {
				t.Errorf("Compile(%.40q).IsMatch(%q) = %v, want %v", c.pattern, subject, got, c.match[i])
			}
			if got := r.ReplaceAll(subject, rep); got != c.want[i] {
				t.Errorf("Compile(%.40q).ReplaceAll(%q) = %q, want %q", c.pattern, subject, got, c.want[i])
			}
		}
	}

	long := strings.Repeat("é", 120)
	large := []struct {
		pattern  string
		match    bool
		replaced string
		err      string
	}{
		{pattern: `\w{100}`, match: true, replaced: "Xéééééééééééééééééééé"},
		{pattern: `\w{500}`, err: sizeError},
		{pattern: `\pL{100}`, match: true, replaced: "Xéééééééééééééééééééé"},
		{pattern: `\p{L}{200}`, replaced: long},
		{pattern: `\p{L}{300}`, err: sizeError},
		{pattern: `[\p{L}\p{N}]{150}`, replaced: long},
		{pattern: `(?-u)\w{100}`, replaced: long},
		{pattern: `(?-u)\w{5000}`, replaced: long},
		{pattern: `.{100}`, match: true, replaced: "Xéééééééééééééééééééé"},
		{pattern: `.{2000}`, replaced: long},
		{pattern: `(?s).{3000}`, replaced: long},
		{pattern: `\d{1000}`, replaced: long},
		{pattern: `\p{Greek}{1000}`, replaced: long},
		{pattern: `[^a]{1000}`, replaced: long},
		{pattern: `a{6,157709}`, err: sizeError},
	}
	for _, c := range large {
		r, err := Compile(c.pattern)
		message := ""
		if err != nil {
			message = err.Error()
		}
		if message != c.err {
			t.Errorf("Compile(%q) error %q, want %q", c.pattern, message, c.err)
		}
		if err != nil {
			continue
		}
		if got := r.IsMatch(long); got != c.match {
			t.Errorf("Compile(%q).IsMatch(120 x é) = %v, want %v", c.pattern, got, c.match)
		}
		if got := r.ReplaceAll(long, "X"); got != c.replaced {
			t.Errorf("Compile(%q).ReplaceAll(120 x é) = %q, want %q", c.pattern, got, c.replaced)
		}
	}
}
