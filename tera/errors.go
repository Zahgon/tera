// Package tera implements the Tera template language.
package tera

import (
	"fmt"
	"strings"
)

// Span locates a source expression. Lines are one based; columns count Unicode scalars.
type Span struct {
	// StartLine is the one-based starting line.
	StartLine int
	// StartCol is the zero-based starting scalar column.
	StartCol int
	// EndLine is the one-based ending line.
	EndLine int
	// EndCol is the zero-based ending scalar column.
	EndCol int
	// Start is the inclusive starting byte offset.
	Start int
	// End is the exclusive ending byte offset.
	End int
}

// String returns the Rust-compatible text representation.
func (s Span) String() string {
	return fmt.Sprintf(" @ %d:%d-%d:%d", s.StartLine, s.StartCol, s.EndLine, s.EndCol)
}
func (s Span) expand(t Span) Span {
	s.EndLine = t.EndLine
	s.EndCol = t.EndCol
	s.End = t.End
	return s
}

type reportNote struct {
	label, filename, source string
	span                    Span
}

// ReportError is an error with source context and optional notes.
type ReportError struct {
	// Message is the diagnostic text without source context.
	Message string
	// Filename identifies the template.
	Filename string
	// Source is the template source.
	Source string
	// Span locates the error.
	Span  Span
	notes []reportNote
}

// Error returns the error message.
func (e ReportError) Error() string { return generateReport(e) }

// ErrorKind identifies a Tera error variant.
type ErrorKind interface {
	fmt.Stringer
	teraErrorKind()
}

// KindMsg holds an unstructured error message.
type KindMsg string

// KindSyntaxError holds a parsing diagnostic.
type KindSyntaxError struct {
	// ReportError holds the source diagnostic.
	ReportError
}

// KindRenderingError holds a rendering diagnostic.
type KindRenderingError struct {
	// ReportError holds the source diagnostic.
	ReportError
}

// KindCircularExtend describes an inheritance cycle.
type KindCircularExtend struct {
	// Tpl identifies the template where inheritance was checked.
	Tpl string
	// InheritanceChain lists visited parents, including the cycle.
	InheritanceChain []string
}

// KindCircularInclude describes an include cycle.
type KindCircularInclude struct {
	// Tpl identifies the repeated template.
	Tpl string
	// IncludeChain lists the include traversal.
	IncludeChain []string
}

// KindMissingParent describes an unknown parent template.
type KindMissingParent struct {
	// Current is the template containing extends.
	Current string
	// Parent is the missing parent name.
	Parent string
}

// KindTemplateNotFound identifies a missing template.
type KindTemplateNotFound string

// KindComponentNotFound identifies a missing component.
type KindComponentNotFound string

// KindInvalidArgument describes an argument type mismatch.
type KindInvalidArgument struct {
	// ExpectedType is the requested Rust argument type.
	ExpectedType string
	// ActualType is the input value kind.
	ActualType string
}

// KindMissingArgument identifies a required keyword argument.
type KindMissingArgument struct {
	// ArgName is the required keyword name.
	ArgName string
}

// KindOutOfRangeArgument describes a numeric conversion overflow.
type KindOutOfRangeArgument struct {
	// Value is the formatted number.
	Value string
	// TargetType is the requested Rust numeric type.
	TargetType string
}

// KindIo describes an output IO error.
type KindIo string

// KindUtf8Conversion describes invalid UTF-8 output.
type KindUtf8Conversion struct{}

// String returns the Rust-compatible text representation.
func (k KindMsg) String() string { return string(k) }

// String returns the Rust-compatible text representation.
func (k KindSyntaxError) String() string { return k.ReportError.Error() }

// String returns the Rust-compatible text representation.
func (k KindRenderingError) String() string { return k.ReportError.Error() }

// String returns the Rust-compatible text representation.
func (k KindCircularExtend) String() string {
	return fmt.Sprintf("Circular extend detected for template '%s'. Inheritance chain: `%s`", k.Tpl, debugStrings(k.InheritanceChain))
}

// String returns the Rust-compatible text representation.
func (k KindCircularInclude) String() string {
	return fmt.Sprintf("Circular include detected for template '%s'. Include chain: `%s`", k.Tpl, debugStrings(k.IncludeChain))
}

// String returns the Rust-compatible text representation.
func (k KindMissingParent) String() string {
	return fmt.Sprintf("Template '%s' is inheriting from '%s', which doesn't exist or isn't loaded.", k.Current, k.Parent)
}

// String returns the Rust-compatible text representation.
func (k KindTemplateNotFound) String() string {
	return fmt.Sprintf("Template '%s' not found", string(k))
}

// String returns the Rust-compatible text representation.
func (k KindComponentNotFound) String() string {
	return fmt.Sprintf("Component '%s' not found", string(k))
}

// String returns the Rust-compatible text representation.
func (k KindInvalidArgument) String() string {
	return fmt.Sprintf("Invalid type for the value, expected `%s` but got `%s`", k.ExpectedType, k.ActualType)
}

// String returns the Rust-compatible text representation.
func (k KindMissingArgument) String() string {
	return fmt.Sprintf("Missing keyword argument `%s`", k.ArgName)
}

// String returns the Rust-compatible text representation.
func (k KindOutOfRangeArgument) String() string {
	return fmt.Sprintf("Value `%s` is out of range for `%s`", k.Value, k.TargetType)
}

// String returns the Rust-compatible text representation.
func (k KindIo) String() string {
	return "Io error while writing rendered value to output: " + string(k)
}

// String returns the Rust-compatible text representation.
func (k KindUtf8Conversion) String() string { return "Invalid UTF-8 characters found while rendering." }

// Error is a Tera error with an optional underlying cause.
type Error struct {
	// Kind identifies the error variant.
	Kind ErrorKind
	// Source is the optional underlying cause.
	Source error
}

// Error returns the error message.
func (e *Error) Error() string { return e.Kind.String() }

// Unwrap returns the underlying cause.
func (e *Error) Unwrap() error { return e.Source }

// NewError constructs an error variant.
func NewError(k ErrorKind) *Error { return &Error{Kind: k} }

// ErrorMessage constructs a plain error.
func ErrorMessage(s string) *Error { return NewError(KindMsg(s)) }

// ErrorChain constructs a plain error with a source.
func ErrorChain(s string, e error) *Error { return &Error{Kind: KindMsg(s), Source: e} }
func invalidArg(t string, v Value) error  { return NewError(KindInvalidArgument{t, v.Name()}) }
func outOfRange(t string, v Value) error  { return NewError(KindOutOfRangeArgument{v.String(), t}) }
func syntaxError(s string, sp Span) *Error {
	return NewError(KindSyntaxError{ReportError: ReportError{Message: s, Span: sp}})
}
func debugStrings(a []string) string {
	b := make([]string, len(a))
	for i, s := range a {
		b[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(b, ", ") + "]"
}

func (KindMsg) teraErrorKind()                {}
func (KindSyntaxError) teraErrorKind()        {}
func (KindRenderingError) teraErrorKind()     {}
func (KindCircularExtend) teraErrorKind()     {}
func (KindCircularInclude) teraErrorKind()    {}
func (KindMissingParent) teraErrorKind()      {}
func (KindTemplateNotFound) teraErrorKind()   {}
func (KindComponentNotFound) teraErrorKind()  {}
func (KindInvalidArgument) teraErrorKind()    {}
func (KindMissingArgument) teraErrorKind()    {}
func (KindOutOfRangeArgument) teraErrorKind() {}
func (KindIo) teraErrorKind()                 {}
func (KindUtf8Conversion) teraErrorKind()     {}
