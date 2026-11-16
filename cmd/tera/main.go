// Command tera renders Tera templates from a directory.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/Keats/tera/internal/rustrt"
	"github.com/Keats/tera/tera"
)

var registerContrib func(*tera.Tera)

type options struct {
	autoescape                                                       []string
	hasAuto                                                          bool
	delimiters                                                       *tera.Delimiters
	prefixes                                                         []string
	globals                                                          [][2]string
	contrib, component, escape, stringMode, variables, componentInfo bool
	block, implicits                                                 string
	body                                                             *string
	positional                                                       []string
	hasBlock, hasImplicits                                           bool
}

func parseArgs(args []string) (options, string) {
	o := options{}
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			o.positional = append(o.positional, a)
			continue
		}
		n, v, has := strings.Cut(a, "=")
		switch n {
		case "--autoescape", "--delimiters", "--fallback-prefix", "--global", "--block", "--implicits", "--body":
			if !has {
				return o, n + " needs a value"
			}
			switch n {
			case "--autoescape":
				o.hasAuto = true
				o.autoescape = []string{}
				if v != "" {
					o.autoescape = strings.Split(v, ",")
				}
			case "--delimiters":
				p := strings.Split(v, ",")
				if len(p) != 6 {
					return o, "--delimiters needs six comma-separated values"
				}
				o.delimiters = &tera.Delimiters{BlockStart: p[0], BlockEnd: p[1], VariableStart: p[2], VariableEnd: p[3], CommentStart: p[4], CommentEnd: p[5]}
			case "--fallback-prefix":
				o.prefixes = append(o.prefixes, v)
			case "--global":
				k, j, ok := strings.Cut(v, "=")
				if !ok {
					return o, "--global needs NAME=JSON"
				}
				o.globals = append(o.globals, [2]string{k, j})
			case "--block":
				o.block = v
				o.hasBlock = true
			case "--implicits":
				o.implicits = v
				o.hasImplicits = true
			case "--body":
				x := v
				o.body = &x
			}
		case "--contrib", "--component", "--escape", "--string", "--variables", "--component-info":
			if has {
				return o, n + " takes no value"
			}
			switch n {
			case "--contrib":
				o.contrib = true
			case "--component":
				o.component = true
			case "--escape":
				o.escape = true
			case "--string":
				o.stringMode = true
			case "--variables":
				o.variables = true
			case "--component-info":
				o.componentInfo = true
			}
		default:
			return o, "unknown option " + n
		}
	}
	if len(o.positional) < 2 || len(o.positional) > 3 {
		return o, "expected TEMPLATE_DIR NAME [CONTEXT_FILE]"
	}
	return o, ""
}

// commandError is a failure of the command itself (exit status 64 for usage, 65 for an unreadable input file); its
// message is printed with the given prefix.
type commandError struct {
	status int
	prefix string
	msg    string
}

func (e *commandError) Error() string { return e.prefix + e.msg }

func usageError(msg string) error {
	return &commandError{status: 64, prefix: "usage error: ", msg: msg}
}

func inputError(msg string) error {
	return &commandError{status: 65, prefix: "input error: ", msg: msg}
}

// writeError reports that the rendered output could not be written to stdout.
type writeError struct{ cause error }

func (e *writeError) Error() string { return "error writing the output: " + e.cause.Error() }

func (e *writeError) Unwrap() error { return e.cause }

// report prints err to stderr as the reference command does (a Tera error followed by one `caused by:` line per
// source) and returns the exit status for its kind.
func report(err error) int {
	var ce *commandError
	if errors.As(err, &ce) {
		fmt.Fprintln(os.Stderr, ce.Error())
		return ce.status
	}
	var we *writeError
	if errors.As(err, &we) {
		fmt.Fprintln(os.Stderr, we.Error())
		return 1
	}
	fmt.Fprintln(os.Stderr, err)
	for s := errors.Unwrap(err); s != nil; s = errors.Unwrap(s) {
		fmt.Fprintln(os.Stderr, "caused by: "+s.Error())
	}
	if er, ok := err.(*tera.Error); ok {
		switch er.Kind.(type) {
		case tera.KindSyntaxError:
			return 2
		case tera.KindRenderingError:
			return 3
		case tera.KindTemplateNotFound:
			return 4
		case tera.KindComponentNotFound:
			return 5
		case tera.KindCircularExtend, tera.KindCircularInclude, tera.KindMissingParent:
			return 6
		}
	}
	return 1
}
func readContext(path string) (*tera.Context, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, fmt.Errorf("%s: %s", path, rustIO(e))
	}
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("%s: stream did not contain valid UTF-8", path)
	}
	v, e := tera.ValueFromJSON(b)
	if e != nil {
		return nil, fmt.Errorf("%s: %s", path, e)
	}
	return tera.ContextFromValue(v)
}
func rustIO(e error) string {
	if os.IsNotExist(e) {
		return "No such file or directory (os error 2)"
	}
	if os.IsPermission(e) {
		return "Permission denied (os error 13)"
	}
	if errors.Is(e, syscall.ENOTDIR) {
		return "Not a directory (os error 20)"
	}
	if errors.Is(e, syscall.EISDIR) {
		return "Is a directory (os error 21)"
	}
	return e.Error()
}
func walkTemplates(dir, base string, out *[]tera.RawTemplate, ancestors map[string]bool) error {
	entries, e := os.ReadDir(dir)
	if e != nil {
		return fmt.Errorf("%s: %s", dir, rustIO(e))
	}
	real, e := filepath.EvalSymlinks(dir)
	if e != nil {
		return fmt.Errorf("%s: %s", dir, rustIO(e))
	}
	if ancestors[real] {
		return nil
	}
	ancestors[real] = true
	defer delete(ancestors, real)
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if info, e := os.Stat(path); e == nil && info.IsDir() {
			if e := walkTemplates(path, base, out, ancestors); e != nil {
				return e
			}
			continue
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return fmt.Errorf("%s: %s", path, rustIO(e))
		}
		if !utf8.Valid(b) {
			return fmt.Errorf("%s: stream did not contain valid UTF-8", path)
		}
		rel, e := filepath.Rel(base, path)
		if e != nil {
			return e
		}
		*out = append(*out, tera.RawTemplate{Name: rustrt.LossyUTF8([]byte(filepath.ToSlash(rel))), Content: string(b)})
	}
	return nil
}
func run(o options) error {
	dir, name := o.positional[0], o.positional[1]
	c := tera.NewContext()
	if len(o.positional) == 3 {
		var e error
		c, e = readContext(o.positional[2])
		if e != nil {
			return inputError(e.Error())
		}
	}
	templates := []tera.RawTemplate{}
	e := walkTemplates(dir, dir, &templates, map[string]bool{})
	if e != nil {
		return inputError(e.Error())
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].Name < templates[j].Name })
	var oneoff *string
	if o.stringMode {
		for i, t := range templates {
			if t.Name == name {
				x := t.Content
				oneoff = &x
				templates = append(templates[:i], templates[i+1:]...)
				break
			}
		}
		if oneoff == nil {
			return inputError(name + ": no such file in the template directory")
		}
	}
	t := tera.New()
	if o.contrib {
		if registerContrib == nil {
			return usageError("--contrib is unavailable")
		}
		registerContrib(t)
	}
	if o.hasAuto {
		t.AutoescapeOn(o.autoescape)
	}
	if o.delimiters != nil {
		if e := t.SetDelimiters(*o.delimiters); e != nil {
			return e
		}
	}
	if len(o.prefixes) > 0 {
		if e := t.SetFallbackPrefixes(o.prefixes); e != nil {
			return e
		}
	}
	for _, g := range o.globals {
		v, e := tera.ValueFromJSON([]byte(g[1]))
		if e != nil {
			return inputError("--global " + g[0] + ": " + e.Error())
		}
		t.GlobalContext().InsertValue(g[0], v)
	}
	if e := t.AddRawTemplates(templates); e != nil {
		return e
	}
	var out string
	switch {
	case oneoff != nil:
		out, e = t.RenderStr(*oneoff, c, o.escape)
	case o.variables:
		var vs []string
		vs, e = t.GetTemplateVariables(name)
		for _, v := range vs {
			out += v + "\n"
		}
	case o.componentInfo:
		out, e = componentInfo(t, name)
	case o.component:
		if o.hasImplicits {
			im, er := readContext(o.implicits)
			if er != nil {
				return inputError(er.Error())
			}
			out, e = t.RenderComponentWithImplicits(name, c, im, o.body, o.escape)
		} else {
			out, e = t.RenderComponent(name, c, o.body, o.escape)
		}
	case o.hasBlock:
		out, e = t.RenderBlock(name, o.block, c)
	default:
		out, e = t.Render(name, c)
	}
	if e != nil {
		return e
	}
	if _, e = os.Stdout.WriteString(out); e != nil {
		return &writeError{cause: e}
	}
	return nil
}
func componentInfo(t *tera.Tera, n string) (string, error) {
	i, ok := t.GetComponentDefinition(n)
	if !ok {
		return "", tera.NewError(tera.KindComponentNotFound(n))
	}
	out := "component " + i.Name() + "\n"
	for _, a := range i.Args() {
		typ := "-"
		if t, ok := a.ArgType(); ok {
			typ = t.AsStr()
		}
		d := "-"
		if v, ok := a.Default(); ok {
			d, _ = tera.MarshalJSON(v, false)
		}
		out += fmt.Sprintf("arg %s type=%s required=%t implicit=%t default=%s\n", a.Name(), typ, a.IsRequired(), a.IsImplicit(), d)
	}
	rest := "-"
	if r, ok := i.RestParam(); ok {
		rest = r
	}
	out += "rest " + rest + "\n"
	for _, p := range i.Metadata().Iter() {
		v, _ := tera.MarshalJSON(p.Value, false)
		out += "meta " + p.Key.String() + " " + v + "\n"
	}
	return out, nil
}
func main() {
	o, msg := parseArgs(os.Args[1:])
	err := usageError(msg)
	if msg == "" {
		err = run(o)
	}
	if err != nil {
		os.Exit(report(err))
	}
}
