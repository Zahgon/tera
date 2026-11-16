package tera

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Keats/tera/internal/rustrt"
)

// Tera stores templates and registered language extensions.
type Tera struct {
	templates          map[string]*template
	components         map[string]*componentDef
	filters            map[string]Filter
	tests              map[string]Test
	functions          map[string]Function
	globals            *Context
	delimiters         Delimiters
	suffixes, prefixes []string
	escapeFn           EscapeFn
	glob               string
	globNames          map[string]bool
}

// New constructs an engine with all core filters, tests, and functions.
func New() *Tera {
	t := &Tera{templates: map[string]*template{}, components: map[string]*componentDef{}, filters: map[string]Filter{}, tests: map[string]Test{}, functions: map[string]Function{}, globals: NewContext(), delimiters: DefaultDelimiters(), suffixes: []string{".html", ".htm", ".xml"}, escapeFn: EscapeHTML, globNames: map[string]bool{}}
	t.registerBuiltins()
	return t
}

// AutoescapeOn replaces autoescaped suffixes.
func (t *Tera) AutoescapeOn(suffixes []string) { t.suffixes = append([]string{}, suffixes...) }

// SetDelimiters configures syntax before templates are loaded.
func (t *Tera) SetDelimiters(d Delimiters) error {
	if len(t.templates) > 0 {
		return ErrorMessage("Delimiters cannot be modified if templates have already been added")
	}
	if e := d.validate(); e != nil {
		return e
	}
	t.delimiters = d
	return nil
}

// SetEscapeFn configures output escaping.
func (t *Tera) SetEscapeFn(fn EscapeFn) { t.escapeFn = fn }

// ResetEscapeFn restores HTML escaping.
func (t *Tera) ResetEscapeFn() { t.escapeFn = EscapeHTML }

// RegisterFilter adds or replaces a filter.
func (t *Tera) RegisterFilter(n string, f Filter) {
	if safe, ok := f.(SafeOutput); ok && safe.IsSafe() {
		original := f
		f = FilterFunc(func(v Value, k Kwargs, s *State) (Value, error) {
			r, e := original.Call(v, k, s)
			return r.MarkSafe(), e
		})
	}
	t.filters[n] = f
}

// RegisterTest adds or replaces a test.
func (t *Tera) RegisterTest(n string, f Test) { t.tests[n] = f }

// RegisterFunction adds or replaces a function.
func (t *Tera) RegisterFunction(n string, f Function) {
	if safe, ok := f.(SafeOutput); ok && safe.IsSafe() {
		original := f
		f = FunctionFunc(func(k Kwargs, s *State) (Value, error) { r, e := original.Call(k, s); return r.MarkSafe(), e })
	}
	t.functions[n] = f
}

// RegisterFrom copies extensions whose names are absent in this engine.
func (t *Tera) RegisterFrom(o *Tera) {
	for n, f := range o.filters {
		if _, ok := t.filters[n]; !ok {
			t.filters[n] = f
		}
	}
	for n, f := range o.tests {
		if _, ok := t.tests[n]; !ok {
			t.tests[n] = f
		}
	}
	for n, f := range o.functions {
		if _, ok := t.functions[n]; !ok {
			t.functions[n] = f
		}
	}
}

// RawTemplate holds a template's name and source.
type RawTemplate struct {
	// Name identifies the template.
	Name string
	// Content is the source text.
	Content string
}

// TemplateFile holds a path and optional template name.
type TemplateFile struct {
	// Path is the input filename.
	Path string
	// Name optionally overrides Path as the template name.
	Name string
}

// AddRawTemplate adds and validates a template atomically.
func (t *Tera) AddRawTemplate(n, c string) error { return t.AddRawTemplates([]RawTemplate{{n, c}}) }

// AddRawTemplates parses and validates a batch atomically.
func (t *Tera) AddRawTemplates(a []RawTemplate) error {
	old := t.templates
	t.templates = map[string]*template{}
	for k, v := range old {
		t.templates[k] = v
	}
	for _, r := range a {
		x, e := parseTemplate(r.Name, r.Content, t.delimiters)
		if e != nil {
			t.templates = old
			return e
		}
		t.templates[r.Name] = x
	}
	if e := t.finalize(); e != nil {
		t.templates = old
		return e
	}
	for _, r := range a {
		delete(t.globNames, r.Name)
	}
	return nil
}

// AddTemplateFile reads and adds a file, using path when name is empty.
func (t *Tera) AddTemplateFile(path, name string) error {
	return t.AddTemplateFiles([]TemplateFile{{path, name}})
}

// AddTemplateFiles reads, parses, and validates files atomically.
func (t *Tera) AddTemplateFiles(files []TemplateFile) error {
	old := t.templates
	t.templates = map[string]*template{}
	for n, tp := range old {
		t.templates[n] = tp
	}
	for _, file := range files {
		tp, e := t.readTemplateFile(file)
		if e != nil {
			t.templates = old
			return e
		}
		t.templates[tp.name] = tp
	}
	if e := t.finalize(); e != nil {
		t.templates = old
		return e
	}
	for _, file := range files {
		name := file.Name
		if name == "" {
			name = file.Path
		}
		delete(t.globNames, name)
	}
	return nil
}
func (t *Tera) readTemplateFile(file TemplateFile) (*template, error) {
	if !utf8.ValidString(file.Path) {
		return nil, ErrorMessage("Template path is not valid UTF-8: " + rustrt.DebugPath(file.Path))
	}
	pathDebug := rustrt.DebugPath(file.Path)
	f, e := os.Open(file.Path)
	if e != nil {
		return nil, ErrorChain("Couldn't open template '"+pathDebug+"'", rustIOError(e))
	}
	b, e := io.ReadAll(f)
	if closeErr := f.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return nil, ErrorChain("Failed to read template '"+pathDebug+"'", rustIOError(e))
	}
	if !utf8.Valid(b) {
		return nil, ErrorChain("Failed to read template '"+pathDebug+"'", fmt.Errorf("stream did not contain valid UTF-8"))
	}
	name := file.Name
	if name == "" {
		name = file.Path
	}
	return parseTemplate(name, string(b), t.delimiters)
}

// SetFallbackPrefixes sets template lookup prefixes before loading templates.
func (t *Tera) SetFallbackPrefixes(p []string) error {
	if len(t.templates) > 0 {
		return ErrorMessage("set_fallback_prefixes must be called before adding templates")
	}
	t.prefixes = append([]string{}, p...)
	return nil
}
func (t *Tera) resolve(n string) *template {
	if x, ok := t.templates[n]; ok {
		return x
	}
	for _, p := range t.prefixes {
		if x, ok := t.templates[p+n]; ok {
			return x
		}
	}
	return nil
}

// ContainsTemplate checks template lookup including fallback prefixes.
func (t *Tera) ContainsTemplate(n string) bool { return t.resolve(n) != nil }

// GetTemplateNames returns registered template names in sorted order.
func (t *Tera) GetTemplateNames() []string {
	a := []string{}
	for k := range t.templates {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}

// ContainsComponent checks for a registered component.
func (t *Tera) ContainsComponent(n string) bool { _, ok := t.components[n]; return ok }

// GetComponentNames returns registered component names in sorted order.
func (t *Tera) GetComponentNames() []string {
	a := []string{}
	for k := range t.components {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}

// GetComponentDefinition returns public component metadata.
func (t *Tera) GetComponentDefinition(n string) (ComponentInfo, bool) {
	d, ok := t.components[n]
	if !ok {
		return ComponentInfo{}, false
	}
	return d.info, true
}

// GlobalContext returns the mutable global context.
func (t *Tera) GlobalContext() *Context { return t.globals }
func (t *Tera) autoescape(n string) bool {
	for _, s := range t.suffixes {
		if strings.HasSuffix(n, s) {
			return true
		}
	}
	return false
}
func (t *Tera) state(tp *template, c *Context, auto bool) *State {
	s := NewState(c)
	s.tera = t
	s.tpl = tp
	s.root = tp
	s.globals = t.globals
	s.autoescape = auto
	s.escapeFn = t.escapeFn
	return s
}
func (t *Tera) render(tp *template, c *Context, auto bool, block string) (string, error) {
	var b strings.Builder
	if e := t.renderTo(tp, c, auto, block, &b); e != nil {
		return "", e
	}
	if e := validateRendered(b.String()); e != nil {
		return "", e
	}
	return b.String(), nil
}
func (t *Tera) renderTo(tp *template, c *Context, auto bool, block string, w io.Writer) error {
	s := t.state(tp, c, auto)
	s.captureBlock = block
	for tp.parent != "" {
		tp = t.resolve(tp.parent)
		if tp == nil {
			return ErrorMessage("Missing parent template")
		}
	}
	s.tpl = tp
	target := w
	if block != "" {
		target = io.Discard
	}
	if _, e := s.renderNodes(tp.nodes, target); e != nil {
		return e
	}
	if block != "" {
		return writeRaw(w, s.captured)
	}
	return nil
}

// Render renders a named template.
func (t *Tera) Render(n string, c *Context) (string, error) {
	tp := t.resolve(n)
	if tp == nil {
		return "", NewError(KindTemplateNotFound(n))
	}
	return t.render(tp, c, t.autoescape(tp.name), "")
}

// RenderTo writes a named template to a writer.
func (t *Tera) RenderTo(n string, c *Context, w io.Writer) error {
	tp := t.resolve(n)
	if tp == nil {
		return NewError(KindTemplateNotFound(n))
	}
	return t.renderTo(tp, c, t.autoescape(tp.name), "", w)
}

// RenderStr parses and renders a one-off template.
func (t *Tera) RenderStr(input string, c *Context, auto bool) (string, error) {
	tp, e := t.prepareString(input)
	if e != nil {
		return "", e
	}
	return t.render(tp, c, auto, "")
}
func (t *Tera) prepareString(input string) (*template, error) {
	tp, e := parseTemplate("__tera_one_off", input, t.delimiters)
	if e != nil {
		return nil, e
	}
	if tp.parent != "" {
		return nil, ErrorMessage("Template inheritance ({% extends %}) is not supported in render_str.")
	}
	if len(tp.blocks) > 0 {
		return nil, ErrorMessage("Blocks not supported in render_str.")
	}
	es := t.validate(tp, t.components)
	if len(es) > 0 {
		return nil, validationError(es)
	}
	return tp, nil
}

// RenderStrTo writes a one-off rendering to a writer.
func (t *Tera) RenderStrTo(input string, c *Context, auto bool, w io.Writer) error {
	tp, e := t.prepareString(input)
	if e != nil {
		return e
	}
	return t.renderTo(tp, c, auto, "", w)
}

// OneOff renders a template using a fresh engine.
func OneOff(input string, c *Context, auto bool) (string, error) {
	return New().RenderStr(input, c, auto)
}

// RenderBlock renders a named block while executing its template.
func (t *Tera) RenderBlock(tpl, block string, c *Context) (string, error) {
	tp := t.resolve(tpl)
	if tp == nil {
		return "", NewError(KindTemplateNotFound(tpl))
	}
	if len(t.lineage(tp, block)) == 0 {
		return "", ErrorMessage("Block `" + block + "` not found in template `" + tpl + "`")
	}
	return t.render(tp, c, t.autoescape(tp.name), block)
}

// RenderBlockTo writes a block rendering to a writer.
func (t *Tera) RenderBlockTo(tpl, block string, c *Context, w io.Writer) error {
	tp := t.resolve(tpl)
	if tp == nil {
		return NewError(KindTemplateNotFound(tpl))
	}
	if len(t.lineage(tp, block)) == 0 {
		return ErrorMessage("Block `" + block + "` not found in template `" + tpl + "`")
	}
	return t.renderTo(tp, c, t.autoescape(tp.name), block, w)
}

// RenderComponent renders a component with explicit arguments.
func (t *Tera) RenderComponent(n string, c *Context, body *string, auto bool) (string, error) {
	return t.RenderComponentWithImplicits(n, c, NewContext(), body, auto)
}

// RenderComponentTo writes a component rendering to a writer.
func (t *Tera) RenderComponentTo(n string, c *Context, body *string, auto bool, w io.Writer) error {
	return t.RenderComponentWithImplicitsTo(n, c, NewContext(), body, auto, w)
}

// RenderComponentWithImplicits renders a component with explicit and implicit contexts.
func (t *Tera) RenderComponentWithImplicits(n string, args, implicits *Context, body *string, auto bool) (string, error) {
	var b strings.Builder
	if e := t.RenderComponentWithImplicitsTo(n, args, implicits, body, auto, &b); e != nil {
		return "", e
	}
	if e := validateRendered(b.String()); e != nil {
		return "", e
	}
	return b.String(), nil
}

// RenderComponentWithImplicitsTo writes a component rendering with implicit arguments.
func (t *Tera) RenderComponentWithImplicitsTo(n string, args, implicits *Context, body *string, auto bool, w io.Writer) error {
	d, ok := t.components[n]
	if !ok {
		return NewError(KindComponentNotFound(n))
	}
	parent := NewState(implicits)
	parent.globals = t.globals
	var v *Value
	if body != nil {
		x := SafeString(*body)
		v = &x
	}
	if args == nil {
		args = NewContext()
	}
	m, _ := args.value().AsMap()
	c, e := d.buildContext(m, parent, v)
	if e != nil {
		return e
	}
	s := t.state(d.tpl, c, auto)
	s.globals = nil
	s.componentParent = parent
	_, e = s.renderNodes(d.body, w)
	return e
}

func (t *Tera) lineage(tp *template, name string) []*template {
	a := []*template{}
	for tp != nil {
		if _, ok := tp.blocks[name]; ok {
			a = append(a, tp)
		}
		if tp.parent == "" {
			break
		}
		tp = t.resolve(tp.parent)
	}
	return a
}
