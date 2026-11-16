package tera

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

const snapshotRoot = "testdata/snapshot_tests"

func readTestFile(t testing.TB, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// Insta stores metadata between two YAML document markers and normalizes the
// final newline independently of the rendered value.
func readSnapshot(t testing.TB, path string) (string, string) {
	t.Helper()
	s := readTestFile(t, path)
	header, body, ok := strings.Cut(strings.TrimPrefix(s, "---\n"), "\n---\n")
	if !ok {
		t.Fatalf("invalid insta snapshot %s", path)
	}
	input := ""
	for _, line := range strings.Split(header, "\n") {
		if strings.HasPrefix(line, "input_file: ") {
			input = strings.TrimPrefix(line, "input_file: tera/src/snapshot_tests/")
		}
	}
	return input, strings.TrimSuffix(body, "\n")
}

func assertSnapshot(t testing.TB, got, want string) {
	t.Helper()
	// Insta 1.48 TextSnapshotContents.normalize trims trailing whitespace.
	got = strings.TrimRightFunc(strings.ReplaceAll(got, "\r\n", "\n"), unicode.IsSpace)
	want = strings.TrimRightFunc(want, unicode.IsSpace)
	if got != want {
		t.Errorf("snapshot mismatch\nexpected:\n%s\ngot:\n%s", want, got)
	}
}

func splitMultiTemplates(body string) []RawTemplate {
	var out []RawTemplate
	for _, part := range strings.Split(body, "$$ ")[1:] {
		name, content, _ := strings.Cut(part, "\n")
		out = append(out, RawTemplate{Name: name, Content: strings.TrimSpace(content)})
	}
	return out
}

func snapshotContext() *Context {
	c := NewContext()
	values := map[string]any{
		"name": "Bob", "description": "<p>I should be escaped by default</p>", "some_html": "<p>Some HTML chars & more</p>",
		"age": 18, "some_bool": true, "one": 1, "product": map[string]any{"name": "Moto G"},
		"vectors": [][]int{{0, 3, 6}, {1, 4, 7}}, "numbers": []int{1, 2, 3}, "empty": []int{},
		"objects": []any{map[string]any{"label": "Child", "parent": map[string]any{"label": "Parent", "parent": nil, "numbers": []int{1, 2, 3}}, "numbers": []int{1, 2, 3}}},
		"data":    map[string]any{"names": []string{"Tchoupi", "Pilou", "Fanny"}, "weights": []float64{50.6, 70.1}},
		"reviews": []any{map[string]any{"title": "My review", "paragraphs": []string{"A", "B", "C"}}, map[string]any{"title": "My review", "paragraphs": []string{"A", "B", "C"}}},
		"to":      "&", "malicious": "<html>",
		"year_data": []any{map[string]any{"id": 1, "year": 2015}, map[string]any{"id": 2, "year": 2015}, map[string]any{"id": 3, "year": 2016}, map[string]any{"id": 4, "year": 2017}, map[string]any{"id": 5, "year": 2018}, map[string]any{"id": 6, "year": nil}, map[string]any{"id": 7, "year": 2018}},
	}
	for k, v := range values {
		c.Insert(k, v)
	}
	c.InsertValue("bytes", Bytes([]byte("hello")))
	return c
}

func registerReadContext(engine *Tera) {
	engine.RegisterFilter("read_ctx", TypedFilter(ArgStr, func(path string, _ Kwargs, state *State) (Value, error) {
		start, rest, nested := strings.Cut(path, ".")
		v, ok := state.Get(start)
		if !ok {
			return Undefined(), nil
		}
		if nested {
			v, ok = v.GetFromPath(rest)
			if !ok {
				return Undefined(), nil
			}
		}
		return v, nil
	}))
}

func TestRenderingSnapshots(t *testing.T) {
	paths, err := filepath.Glob(snapshotRoot + "/snapshots/*__rendering__*.snap")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		input, want := readSnapshot(t, path)
		if input == "" {
			continue
		} // Standalone cases are exercised below.
		t.Run(strings.TrimSuffix(filepath.Base(path), ".snap"), func(t *testing.T) {
			if strings.HasSuffix(input, "/success/spread.txt") {
				// The original records rendering_ok without preserve_order and disables it with that feature; this
				// port keeps insertion order like the preserve_order build, so the printed spread maps differ.
				t.Skip("snapshot recorded with the unordered map build")
			}
			src := readTestFile(t, filepath.Join(snapshotRoot, input))
			engine := New()
			name := filepath.Base(input)
			var templates []RawTemplate
			if strings.Contains(src, "$$ ") {
				templates = splitMultiTemplates(src)
				name = templates[len(templates)-1].Name
				engine.AutoescapeOn([]string{".html"})
				engine.GlobalContext().Insert("global_lang", "fr")
			} else {
				templates = []RawTemplate{{Name: name, Content: src}}
				if strings.Contains(input, "/success/") {
					engine.AutoescapeOn([]string{".txt"})
					registerReadContext(engine)
				}
			}
			if err := engine.AddRawTemplates(templates); err != nil {
				t.Fatalf("building fixture: %v", err)
			}
			got, err := engine.Render(name, snapshotContext())
			if strings.Contains(input, "/errors/") {
				if err == nil {
					t.Fatalf("expected rendering error; got %q", got)
				}
				assertSnapshot(t, err.Error(), want)
			} else {
				if err != nil {
					t.Fatal(err)
				}
				assertSnapshot(t, got, want)
			}
		})
	}
}

func TestBuildErrorSnapshots(t *testing.T) {
	paths, err := filepath.Glob(snapshotRoot + "/snapshots/*__build_errors__*.snap")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		input, want := readSnapshot(t, path)
		t.Run(strings.TrimSuffix(filepath.Base(path), ".snap"), func(t *testing.T) {
			engine := New()
			err := engine.AddRawTemplates(splitMultiTemplates(readTestFile(t, filepath.Join(snapshotRoot, input))))
			if err == nil {
				t.Fatal("expected build error")
			}
			assertSnapshot(t, err.Error(), want)
		})
	}
}

func TestLexerSnapshots(t *testing.T) {
	err := filepath.WalkDir(snapshotRoot+"/lexer_inputs", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		t.Run(strings.TrimPrefix(path, snapshotRoot+"/lexer_inputs/"), func(t *testing.T) {
			tokens := lex(readTestFile(t, path), DefaultDelimiters())
			var lexErr error
			for _, tok := range tokens {
				if tok.err != nil {
					lexErr = tok.err
					break
				}
			}
			if strings.Contains(filepath.ToSlash(path), "/errors/") {
				if lexErr == nil {
					t.Fatal("expected lexer error from Rust error snapshot")
				}
				_, want := readSnapshot(t, snapshotRoot+"/snapshots/tera__snapshot_tests__lexer__lexer_errors@"+filepath.Base(path)+".snap")
				var message string
				var span Span
				for _, line := range strings.Split(want, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "message: ") {
						var err error
						message, err = strconv.Unquote(strings.TrimSuffix(strings.TrimPrefix(line, "message: "), ","))
						if err != nil {
							t.Fatal(err)
						}
					}
					if strings.HasPrefix(line, "span: ") {
						_, err := fmt.Sscanf(line, "span: @ %d:%d-%d:%d (%d..%d),", &span.StartLine, &span.StartCol, &span.EndLine, &span.EndCol, &span.Start, &span.End)
						if err != nil {
							t.Fatal(err)
						}
					}
				}
				var engineErr *Error
				if !errors.As(lexErr, &engineErr) {
					t.Fatalf("expected Tera syntax error, got %v", lexErr)
				}
				kind, ok := engineErr.Kind.(KindSyntaxError)
				if !ok || kind.Message != message || kind.Span != span {
					t.Errorf("expected lexer message %q at %+v, got %v", message, span, lexErr)
				}

			} else if lexErr != nil {
				t.Fatal(lexErr)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestParserSnapshots(t *testing.T) {
	err := filepath.WalkDir(snapshotRoot+"/parser_inputs", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		t.Run(strings.TrimPrefix(path, snapshotRoot+"/parser_inputs/"), func(t *testing.T) {
			_, err := parseTemplate(filepath.Base(path), readTestFile(t, path), DefaultDelimiters())
			if strings.Contains(filepath.ToSlash(path), "/errors/") {
				if err == nil {
					t.Fatal("expected parser error")
				}
				_, want := readSnapshot(t, snapshotRoot+"/snapshots/tera__snapshot_tests__parser__parser_errors@"+filepath.Base(path)+".snap")
				assertSnapshot(t, err.Error(), want)
			} else if err != nil {
				t.Fatal(err)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompilerSnapshots(t *testing.T) {
	// Go executes the parsed AST directly, so parsing is its compilation stage.
	err := filepath.WalkDir(snapshotRoot+"/compiler_inputs", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		t.Run(strings.TrimPrefix(path, snapshotRoot+"/compiler_inputs/"), func(t *testing.T) {
			if _, err := parseTemplate(filepath.Base(path), readTestFile(t, path), DefaultDelimiters()); err != nil {
				t.Fatal(err)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderingCustomDelimiters(t *testing.T) {
	engine := New()
	if err := engine.SetDelimiters(Delimiters{"<%", "%>", "<<", ">>", "<#", "#>"}); err != nil {
		t.Fatal(err)
	}
	src := `Hello, << name >>!
<% if some_bool %>Bool is true<% endif %>
<% for num in numbers %>[<< num >>]<% endfor %>
<# This comment should not appear #>
Age: << age >>
<<- " trimmed " ->>
<% raw %><<not a variable>><% endraw %>`
	if err := engine.AddRawTemplate("custom_delimiters.txt", src); err != nil {
		t.Fatal(err)
	}
	got, err := engine.Render("custom_delimiters.txt", snapshotContext())
	if err != nil {
		t.Fatal(err)
	}
	_, want := readSnapshot(t, snapshotRoot+"/snapshots/tera__snapshot_tests__rendering__rendering_custom_delimiters.snap")
	assertSnapshot(t, got, want)
}

func TestRenderStrErrors(t *testing.T) {
	c := NewContext()
	c.Insert("x", 1)
	for _, src := range []string{`{{ youtube() }}`, `{{ "hello" | unknown_filter }}`, `{% if x is unknown_test %}yes{% endif %}`, `{{<Unknown />}}`, `{% include "missing.html" %}`} {
		t.Run(src, func(t *testing.T) {
			if got, err := New().RenderStr(src, c, false); err == nil {
				t.Fatalf("expected error; got %q", got)
			}
		})
	}
}

func TestRenderCaptureBodySafety(t *testing.T) {
	engine := New()
	engine.AutoescapeOn([]string{".html"})
	engine.RegisterFilter("is_safe", FilterFunc(func(v Value, _ Kwargs, _ *State) (Value, error) { return Bool(v.IsSafe()), nil }))
	src := `{% set x %}a & b{% endset %}{{ x | is_safe }}-{{ <hey/> | is_safe }}-{% <ho> %}a & b{% </ho> %}`
	if err := engine.AddRawTemplates([]RawTemplate{{"components.html", `{% component hey() %}a & b{% endcomponent %}{% component ho() %}{{body | is_safe }}{% endcomponent %}`}, {"a.txt", src}, {"a.html", src}}); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"a.txt": "false-false-false", "a.html": "true-true-true"} {
		got, err := engine.Render(name, NewContext())
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s: expected %q, got %q", name, want, got)
		}
	}
}

func TestInlineMapPreserveOrder(t *testing.T) {
	src := `
{% set m = {"name": "Alex", "age": 42, "vip": true, } -%}
{{ m }}
{% for k, v in m -%}
{{ k }} = {{ v }}
{% endfor -%}
`
	engine := New()
	mustAdd(t, engine, RawTemplate{"tpl", src})
	got, err := engine.Render("tpl", NewContext())
	if err != nil {
		t.Fatal(err)
	}
	_, want := readSnapshot(t, snapshotRoot+"/snapshots/tera__snapshot_tests__rendering__inline_map_preserve_order.snap")
	assertSnapshot(t, got, want)
}
