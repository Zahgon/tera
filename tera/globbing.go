package tera

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Keats/tera/internal/rustrt"
)

// GlobFile is a matched template file and its path relative to the glob root.
type GlobFile struct {
	// Path is the matched filesystem path.
	Path string
	// Name is relative to the glob's fixed directory prefix.
	Name string
}

// LoadGlob returns files matching a glob, including recursive ** and brace alternatives.
func LoadGlob(glob string) ([]GlobFile, error) {
	star := strings.IndexByte(glob, '*')
	if star < 0 {
		return nil, ErrorMessage("Not a valid glob: no `*` were found in `" + glob + "`")
	}
	split := strings.LastIndexByte(glob[:star], '/') + 1
	parent, pattern := glob[:split], glob[split:]
	if parent == "" {
		parent = "."
	}
	// A parent that cannot be canonicalised is used as given, as the original does.
	if p, e := filepath.EvalSymlinks(parent); e == nil {
		if abs, e := filepath.Abs(p); e == nil {
			parent = abs
		}
	}
	canonical := strings.TrimSuffix(parent, "/") + "/" + pattern
	re, e := compileGlob(canonical)
	if e != nil {
		return nil, ErrorMessage("Glob is invalid: error parsing glob '" + canonical + "': " + e.Error())
	}

	out := []GlobFile{}
	ancestors := map[string]bool{}
	// Like the original's directory walk, entries that cannot be read (a dangling link, a directory without
	// permission) are skipped rather than failing the whole load: a glob over an unreadable tree matches nothing.
	var walk func(string, string)
	walk = func(path, rel string) {
		info, e := os.Stat(path)
		if e != nil {
			return
		}
		if !info.IsDir() {
			if re.MatchString(globBytes(filepath.ToSlash(path))) {
				out = append(out, GlobFile{path, rustrt.LossyUTF8([]byte(strings.ReplaceAll(filepath.ToSlash(rel), "\\", "/")))})
			}
			return
		}
		real, e := filepath.EvalSymlinks(path)
		if e != nil {
			return
		}
		if ancestors[real] {
			return
		}
		ancestors[real] = true
		defer delete(ancestors, real)
		entries, e := os.ReadDir(path)
		if e != nil {
			return
		}
		for _, entry := range entries {
			walk(filepath.Join(path, entry.Name()), filepath.Join(rel, entry.Name()))
		}
	}
	walk(parent, "")
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// LoadFromGlob reloads glob templates while preserving manually added templates.
func (t *Tera) LoadFromGlob(glob string) error {
	files, e := LoadGlob(glob)
	if e != nil {
		return e
	}
	old, oldNames := t.templates, t.globNames
	t.templates = map[string]*template{}
	for n, tp := range old {
		if !oldNames[n] {
			t.templates[n] = tp
		}
	}
	names := map[string]bool{}
	errors := []string{}
	for _, f := range files {
		tp, e := t.readTemplateFile(TemplateFile{Path: f.Path, Name: f.Name})
		if e != nil {
			errors = append(errors, "Failed to load "+rustrt.LossyUTF8([]byte(f.Path))+": "+e.Error())
			continue
		}
		t.templates[f.Name] = tp
		names[f.Name] = true
	}

	if len(errors) > 0 {
		e = ErrorMessage(strings.Join(errors, "\n"))
	} else {
		e = t.finalize()
	}
	if e != nil {
		t.templates = old
		return e
	}
	t.glob = glob
	t.globNames = names
	return nil
}

// FullReload reloads the last glob, preserving manually added templates.
func (t *Tera) FullReload() error {
	if t.glob == "" {
		return ErrorMessage("Reloading is only available if you are using a glob")
	}
	return t.LoadFromGlob(t.glob)
}

type globToken struct {
	kind    string
	literal rune
	ranges  [][2]rune
	negated bool
	alts    [][]globToken
}
type globParser struct {
	source []rune
	at     int
}

func (g *globParser) peek() rune {
	if g.at >= len(g.source) {
		return 0
	}
	return g.source[g.at]
}
func (g *globParser) branch(nested bool) ([]globToken, error) {
	ts := []globToken{}
	for g.at < len(g.source) {
		c := g.source[g.at]
		if nested && (c == ',' || c == '}') {
			return ts, nil
		}
		prev := rune(0)
		if g.at > 0 {
			prev = g.source[g.at-1]
		}
		g.at++
		switch c {
		case '?':
			ts = append(ts, globToken{kind: "any"})
		case '*':
			if g.peek() != '*' {
				ts = append(ts, globToken{kind: "star"})
				continue
			}
			g.at++
			after := g.peek()
			if len(ts) == 0 {
				if after != 0 && after != '/' {
					ts = append(ts, globToken{kind: "star"}, globToken{kind: "star"})
				} else {
					ts = append(ts, globToken{kind: "prefix"})
					if after == '/' {
						g.at++
					}
				}
				continue
			}
			if prev != '/' && !(nested && (prev == ',' || prev == '{')) {
				ts = append(ts, globToken{kind: "star"}, globToken{kind: "star"})
				continue
			}
			suffix := after == 0 || nested && (after == ',' || after == '}')
			if !suffix && after != '/' {
				ts = append(ts, globToken{kind: "star"}, globToken{kind: "star"})
				continue
			}
			if after == '/' {
				g.at++
			}
			last := ts[len(ts)-1]
			ts = ts[:len(ts)-1]
			if last.kind == "prefix" || last.kind == "suffix" {
				ts = append(ts, last)
			} else if suffix {
				ts = append(ts, globToken{kind: "suffix"})
			} else {
				ts = append(ts, globToken{kind: "recursive"})
			}
		case '{':
			alts := [][]globToken{}
			for {
				a, e := g.branch(true)
				if e != nil {
					return nil, e
				}
				alts = append(alts, a)
				if g.at >= len(g.source) {
					return nil, fmt.Errorf("unclosed alternate group; missing '}' (maybe escape '{' with '[{]'?)")
				}
				end := g.peek()
				g.at++
				if end == '}' {
					break
				}
			}
			ts = append(ts, globToken{kind: "alts", alts: alts})
		case '}':
			return nil, fmt.Errorf("unopened alternate group; missing '{' (maybe escape '}' with '[}]'?)")
		case '[':
			t := globToken{kind: "class"}
			if g.peek() == '!' || g.peek() == '^' {
				t.negated = true
				g.at++
			}
			first, inRange, closed := true, false, false
			for g.at < len(g.source) {
				r := g.peek()
				g.at++
				if r == ']' && !first {
					closed = true
					break
				}
				switch {
				case r == ']':
					t.ranges = append(t.ranges, [2]rune{']', ']'})
				case r == '-' && first:
					t.ranges = append(t.ranges, [2]rune{'-', '-'})
				case r == '-' && !inRange:
					inRange = true
				case inRange:
					last := len(t.ranges) - 1
					low := t.ranges[last][0]
					if r < low {
						return nil, fmt.Errorf("invalid range; '%c' > '%c'", low, r)
					}
					t.ranges[last][1] = r
					inRange = false
				default:
					t.ranges = append(t.ranges, [2]rune{r, r})
				}
				first = false
			}
			if !closed {
				return nil, fmt.Errorf("unclosed character class; missing ']'")
			}
			if inRange {
				t.ranges = append(t.ranges, [2]rune{'-', '-'})
			}
			ts = append(ts, t)
		case '\\':
			if g.at >= len(g.source) {
				return nil, fmt.Errorf("dangling '\\'")
			}
			ts = append(ts, globToken{kind: "literal", literal: g.peek()})
			g.at++
		default:
			ts = append(ts, globToken{kind: "literal", literal: c})
		}
	}
	return ts, nil
}
func globBytes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		b.WriteRune(rune(s[i]))
	}
	return b.String()
}
func globLiteral(r rune) string {
	var b strings.Builder
	for _, c := range []byte(string(r)) {
		fmt.Fprintf(&b, `\x{%x}`, c)
	}
	return b.String()
}
func globRegex(a []globToken) string {
	var b strings.Builder
	for _, t := range a {
		switch t.kind {
		case "literal":
			b.WriteString(globLiteral(t.literal))
		case "any":
			b.WriteString("[^/]")
		case "star":
			b.WriteString("[^/]*")
		case "prefix":
			b.WriteString("(?:/?|.*/)")
		case "suffix":
			b.WriteString("/.*")
		case "recursive":
			b.WriteString("(?:/|/.*/)")
		case "class":
			b.WriteByte('[')
			if t.negated {
				b.WriteByte('^')
			}
			for _, p := range t.ranges {
				b.WriteString(globLiteral(p[0]))
				if p[0] != p[1] {
					b.WriteByte('-')
					b.WriteString(globLiteral(p[1]))
				}
			}
			b.WriteByte(']')
		case "alts":
			parts := []string{}
			for _, a := range t.alts {
				s := globRegex(a)
				if s != "" {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				b.WriteString("(?:" + strings.Join(parts, "|") + ")")
			}
		}
	}
	return b.String()
}
func compileGlob(s string) (*regexp.Regexp, error) {
	g := globParser{source: []rune(s)}
	a, e := g.branch(false)
	if e != nil {
		return nil, e
	}
	pattern := globRegex(a)
	if len(a) == 1 && a[0].kind == "prefix" {
		pattern = ".*"
	}
	re, e := regexp.Compile("(?s)^" + pattern + "$")
	return re, e
}

// LoadFromGlob lists files matching a glob without modifying an engine.
func LoadFromGlob(glob string) ([]GlobFile, error) { return LoadGlob(glob) }
