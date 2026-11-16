package tera

import "strings"

// ComponentArgType is a declared component argument type.
type ComponentArgType string

// Component argument types.
const (
	// ComponentString identifies string values.
	ComponentString ComponentArgType = "String"
	// ComponentBool identifies bool values.
	ComponentBool ComponentArgType = "Bool"
	// ComponentInteger identifies integer values.
	ComponentInteger ComponentArgType = "Integer"
	// ComponentFloat identifies float values.
	ComponentFloat ComponentArgType = "Float"
	// ComponentNumber identifies number values.
	ComponentNumber ComponentArgType = "Number"
	// ComponentArray identifies array values.
	ComponentArray ComponentArgType = "Array"
	// ComponentMap identifies map values.
	ComponentMap ComponentArgType = "Map"
	// ComponentBytes identifies bytes values.
	ComponentBytes ComponentArgType = "Bytes"
)

// AsStr returns the language type name.
func (t ComponentArgType) AsStr() string { return strings.ToLower(string(t)) }

// String returns the Rust-compatible text representation.
func (t ComponentArgType) String() string { return strings.ToLower(string(t)) }
func (t ComponentArgType) matches(v Value) bool {
	switch t {
	case ComponentString:
		return v.IsString()
	case ComponentBool:
		return v.IsBool()
	case ComponentInteger:
		return v.IsNumber() && !v.IsF64()
	case ComponentFloat:
		return v.IsF64()
	case ComponentNumber:
		return v.IsNumber()
	case ComponentArray:
		return v.IsArray()
	case ComponentMap:
		return v.IsMap()
	case ComponentBytes:
		return v.IsBytes()
	}
	return false
}
func typeFromValue(v Value) ComponentArgType {
	switch v.kind {
	case KindString:
		return ComponentString
	case KindBool:
		return ComponentBool
	case KindU64, KindI64, KindU128, KindI128:
		return ComponentInteger
	case KindF64:
		return ComponentFloat
	case KindArray:
		return ComponentArray
	case KindMap:
		return ComponentMap
	case KindBytes:
		return ComponentBytes
	}
	return ""
}

// ComponentArg describes a declared argument.
type ComponentArg struct {
	name         string
	defaultValue *Value
	argType      ComponentArgType
	implicit     bool
}

// Name returns the argument name.
func (a ComponentArg) Name() string { return a.name }

// Default returns the default value and whether one was specified.
func (a ComponentArg) Default() (Value, bool) {
	if a.defaultValue == nil {
		return Undefined(), false
	}
	return *a.defaultValue, true
}

// ArgType returns the declared or inferred argument type.
func (a ComponentArg) ArgType() (ComponentArgType, bool) { return a.argType, a.argType != "" }

// IsRequired reports whether an explicit value is required.
func (a ComponentArg) IsRequired() bool { return !a.implicit && a.defaultValue == nil }

// IsImplicit reports whether the argument resolves from the caller.
func (a ComponentArg) IsImplicit() bool { return a.implicit }

// ComponentInfo describes a component definition.
type ComponentInfo struct {
	name      string
	args      []ComponentArg
	restParam string
	metadata  *Map
}

// Name returns the component name.
func (i ComponentInfo) Name() string { return i.name }

// Args returns declared arguments sorted by name.
func (i ComponentInfo) Args() []ComponentArg { return append([]ComponentArg{}, i.args...) }

// RestParam returns the rest parameter name.
func (i ComponentInfo) RestParam() (string, bool) { return i.restParam, i.restParam != "" }

// Metadata returns component metadata sorted by key.
func (i ComponentInfo) Metadata() *Map { return i.metadata.Clone() }

func componentType(s string) ComponentArgType {
	switch strings.ToLower(s) {
	case "string":
		return ComponentString
	case "bool":
		return ComponentBool
	case "integer":
		return ComponentInteger
	case "float":
		return ComponentFloat
	case "number":
		return ComponentNumber
	case "array":
		return ComponentArray
	case "map":
		return ComponentMap
	case "bytes":
		return ComponentBytes
	}
	return ""
}
