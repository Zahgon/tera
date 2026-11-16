package regex

import (
	"regexp/syntax"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

type interval struct{ lo, hi rune }
type runeSet []interval

func normalize(s runeSet) runeSet {
	if len(s) == 0 {
		return nil
	}
	sort.Slice(s, func(i, j int) bool { return s[i].lo < s[j].lo || s[i].lo == s[j].lo && s[i].hi < s[j].hi })
	out := make(runeSet, 0, len(s))
	for _, v := range s {
		if v.lo > v.hi {
			continue
		}
		if len(out) > 0 && v.lo <= out[len(out)-1].hi+1 {
			if v.hi > out[len(out)-1].hi {
				out[len(out)-1].hi = v.hi
			}
		} else {
			out = append(out, v)
		}
	}
	return out
}
func union(a, b runeSet) runeSet { return normalize(append(append(runeSet(nil), a...), b...)) }
func intersection(a, b runeSet) runeSet {
	var out runeSet
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		lo, hi := a[i].lo, a[i].hi
		if b[j].lo > lo {
			lo = b[j].lo
		}
		if b[j].hi < hi {
			hi = b[j].hi
		}
		if lo <= hi {
			out = append(out, interval{lo, hi})
		}
		if a[i].hi < b[j].hi {
			i++
		} else {
			j++
		}
	}
	return out
}
func complement(a runeSet, max rune) runeSet {
	var out runeSet
	lo := rune(0)
	for _, v := range a {
		if v.lo > max {
			break
		}
		if lo < v.lo {
			out = append(out, interval{lo, v.lo - 1})
		}
		lo = v.hi + 1
	}
	if lo <= max {
		out = append(out, interval{lo, max})
	}
	return out
}
func difference(a, b runeSet) runeSet { return intersection(a, complement(b, utf8.MaxRune)) }
func inSet(s runeSet, r rune) bool {
	i := sort.Search(len(s), func(i int) bool { return s[i].hi >= r })
	return i < len(s) && s[i].lo <= r
}
func fromTable(t *unicode.RangeTable) runeSet {
	var out runeSet
	if t == nil {
		return nil
	}
	for _, r := range t.R16 {
		if r.Stride == 1 {
			out = append(out, interval{rune(r.Lo), rune(r.Hi)})
		} else {
			for c := uint32(r.Lo); c <= uint32(r.Hi); c += uint32(r.Stride) {
				out = append(out, interval{rune(c), rune(c)})
			}
		}
	}
	for _, r := range t.R32 {
		if r.Stride == 1 {
			out = append(out, interval{rune(r.Lo), rune(r.Hi)})
		} else {
			for c := r.Lo; c <= r.Hi; c += r.Stride {
				out = append(out, interval{rune(c), rune(c)})
			}
		}
	}
	return normalize(out)
}

var propertyCache sync.Map

// unicodeProperty describes corrections from Go's Unicode 15 tables to Unicode 16.
type unicodeProperty struct {
	base        []string
	add, remove runeSet
}

func propertyKey(s string) string {
	lower := strings.ToLower(s)
	prefix := strings.HasPrefix(lower, "is")
	if prefix {
		s = s[2:]
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c > 127 || c == ' ' || c == '_' || c == '-' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b.WriteByte(c)
	}
	if prefix && b.String() == "c" {
		return "isc"
	}
	return b.String()
}
func unicodeData(key string) (runeSet, bool) {
	if v, ok := propertyCache.Load(key); ok {
		return v.(runeSet), true
	}
	entry, ok := unicodeProperties[key]
	if !ok {
		return nil, false
	}
	var set runeSet
	for _, base := range entry.base {
		pair := strings.SplitN(base, ":", 2)
		var table *unicode.RangeTable
		switch pair[0] {
		case "gc":
			table = unicode.Categories[pair[1]]
		case "script":
			table = unicode.Scripts[pair[1]]
		case "bool":
			table = unicode.Properties[pair[1]]
		}
		set = union(set, fromTable(table))
	}
	set = difference(union(set, entry.add), entry.remove)
	propertyCache.Store(key, set)
	return set, true
}
func property(name string) (runeSet, string) {
	family, value := "", name
	hasFamily := false
	if i := strings.IndexAny(name, "=:"); i >= 0 {
		hasFamily = true
		family = propertyKey(name[:i])
		value = name[i+1:]
	}
	value = propertyKey(value)
	canonical := ""
	if !hasFamily {
		if value != "cf" && value != "sc" && value != "lc" {
			if n, ok := unicodePropertyNames[value]; ok {
				set, ok := unicodeData("property_bool:" + n)
				if !ok {
					return nil, "Unicode property not found"
				}
				return set, ""
			}
		}
		canonical = unicodePropertyValues["generalcategory"][value]
		if value == "ascii" || value == "any" || value == "assigned" {
			canonical = value
		}
		if canonical != "" {
			family = "generalcategory"
		} else {
			canonical = unicodePropertyValues["script"][value]
			family = "script"
		}
		if canonical == "" {
			return nil, "Unicode property not found"
		}
	} else {
		normalized, ok := unicodePropertyNames[family]
		if !ok {
			return nil, "Unicode property not found"
		}
		family = normalized
		canonical = unicodePropertyValues[family][value]
		if family == "generalcategory" && (value == "ascii" || value == "any" || value == "assigned") {
			canonical = value
		}
		if canonical == "" {
			return nil, "Unicode property value not found"
		}
	}
	if family == "generalcategory" {
		switch canonical {
		case "ascii":
			return runeSet{{0, 127}}, ""
		case "any":
			return runeSet{{0, 0xd7ff}, {0xe000, utf8.MaxRune}}, ""
		case "assigned":
			s, _ := unicodeData("general_category:unassigned")
			return difference(complement(s, utf8.MaxRune), runeSet{{0xd800, 0xdfff}}), ""
		}
	}
	familyName := map[string]string{"generalcategory": "general_category", "script": "script", "scriptextensions": "script_extension", "age": "age", "graphemeclusterbreak": "grapheme_cluster_break", "wordbreak": "word_break", "sentencebreak": "sentence_break"}[family]
	if familyName == "" {
		return nil, "Unicode property not found"
	}
	if family == "age" {
		order := []string{"v11", "v20", "v21", "v30", "v31", "v32", "v40", "v41", "v50", "v51", "v52", "v60", "v61", "v62", "v63", "v70", "v80", "v90", "v100", "v110", "v120", "v121", "v130", "v140", "v150", "v151", "v160"}
		var set runeSet
		for _, version := range order {
			s, _ := unicodeData("age:" + version)
			set = union(set, s)
			if version == canonical {
				return difference(set, runeSet{{0xd800, 0xdfff}}), ""
			}
		}
		return nil, "Unicode property value not found"
	}
	set, ok := unicodeData(familyName + ":" + canonical)
	if !ok {
		return nil, "Unicode property value not found"
	}
	return set, ""
}

var wordUnicodeOnce sync.Once
var wordUnicode runeSet

func unicodeWord() runeSet {
	wordUnicodeOnce.Do(func() {
		a, _ := property("Alphabetic")
		m, _ := property("M")
		nd, _ := property("Nd")
		pc, _ := property("Pc")
		j, _ := property("Join_Control")
		wordUnicode = union(union(union(a, m), union(nd, pc)), j)
	})
	return wordUnicode
}
func perl(c byte, ascii bool) runeSet {
	var s runeSet
	switch c | 32 {
	case 'd':
		if ascii {
			s = runeSet{{'0', '9'}}
		} else {
			s, _ = property("Nd")
		}
	case 's':
		if ascii {
			s = runeSet{{9, 13}, {32, 32}}
		} else {
			s, _ = property("White_Space")
		}
	case 'w':
		if ascii {
			s = runeSet{{'0', '9'}, {'A', 'Z'}, {'_', '_'}, {'a', 'z'}}
		} else {
			s = unicodeWord()
		}
	}
	if c >= 'A' && c <= 'Z' {
		max := rune(utf8.MaxRune)
		if ascii {
			max = 255
		}
		s = complement(s, max)
	}
	return s
}
func (p *parser) fold(s runeSet) runeSet {
	if p.flags&flagCase == 0 {
		return s
	}
	out := append(runeSet(nil), s...)
	if p.flags&flagUnicode == 0 {
		for r := rune('A'); r <= 'Z'; r++ {
			if inSet(s, r) || inSet(s, r+32) {
				out = append(out, interval{r, r}, interval{r + 32, r + 32})
			}
		}
	} else {
		caseFoldOnce.Do(func() {
			for c := rune(0); c <= utf8.MaxRune; c++ {
				if unicode.SimpleFold(c) != c {
					caseFoldRunes = append(caseFoldRunes, c)
				}
			}
		})
		for _, c := range caseFoldRunes {
			if inSet(s, c) {
				for next := unicode.SimpleFold(c); next != c; next = unicode.SimpleFold(next) {
					out = append(out, interval{next, next})
				}
			}
		}
	}
	if p.flags&flagUnicode != 0 {
		for c, others := range unicodeCaseFoldSupplement {
			if inSet(s, c) {
				for _, r := range others {
					out = append(out, interval{r, r})
				}
			}
		}
	}
	return normalize(out)
}
func (p *parser) classNode(s runeSet, start, end int) *syntax.Regexp {
	if p.flags&flagUnicode == 0 {
		for _, v := range s {
			if v.hi >= 128 {
				p.translation(start, end, "pattern can match invalid UTF-8")
				break
			}
		}
	}
	s = difference(s, runeSet{{0xd800, 0xdfff}})
	if len(s) == 0 {
		return &syntax.Regexp{Op: syntax.OpNoMatch}
	}
	rs := make([]rune, 0, len(s)*2)
	for _, v := range s {
		rs = append(rs, v.lo, v.hi)
	}
	return &syntax.Regexp{Op: syntax.OpCharClass, Rune: rs}
}

var caseFoldOnce sync.Once
var caseFoldRunes []rune
