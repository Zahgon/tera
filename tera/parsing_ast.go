package tera

type expr struct {
	k, name, op        string
	v                  Value
	sp                 Span
	left, right, third *expr
	items              []*expr
	entries            []exprEntry
	kwargs             []namedExpr
	optional           bool
	body               []*node
	key, val           string
	selfClosing        bool
}
type namedExpr struct {
	name string
	e    *expr
}
type exprEntry struct {
	key    Key
	value  *expr
	spread bool
}
type node struct {
	k, text, name, key string
	sp                 Span
	e                  *expr
	body, other        []*node
	filters            []*expr
	global             bool
}
type template struct {
	name, source, parent string
	nodes                []*node
	blocks               map[string]*node
	components           map[string]*componentDef
	includes             []namedSpan
}
type namedSpan struct {
	name string
	sp   Span
}
type componentDef struct {
	info ComponentInfo
	body []*node
	tpl  *template
	sp   Span
}
