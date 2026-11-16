package contrib

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Keats/tera/tera"
	"github.com/Keats/tera/tera-contrib/internal/formatx"
)

// maxSpecNum bounds every number in a format spec, since a huge width would allocate a huge string.
const maxSpecNum = 100

// Format formats a value with a Rust std::fmt format spec given in the mandatory `spec` argument: width,
// alignment, fill, precision, sign and zero padding of the Display form. Strings, integers, floats and booleans
// are accepted.
//
//	{{ 3.14159 | format(spec=".2") }} -> 3.14
//	{{ 42 | format(spec="05") }} -> 00042
var Format = tera.TypedFilter(tera.ArgValue, func(val tera.Value, kwargs tera.Kwargs, _ *tera.State) (tera.Value, error) {
	spec, err := tera.KwargsMustGet(kwargs, "spec", tera.ArgStr)
	if err != nil {
		return tera.Undefined(), err
	}
	if strings.Contains(spec, "$") {
		return tera.Undefined(), tera.ErrorMessage(fmt.Sprintf("format spec `%s` cannot reference arguments with `$`", spec))
	}
	digits := strings.FieldsFunc(spec, func(r rune) bool { return r < '0' || r > '9' })
	for _, d := range digits {
		if n, err := strconv.ParseUint(d, 10, 64); err != nil || n > maxSpecNum {
			return tera.Undefined(), tera.ErrorMessage(fmt.Sprintf(
				"format spec `%s` contains a number bigger than the maximum allowed (%d)", spec, maxSpecNum))
		}
	}
	var out string
	switch val.Kind() {
	case tera.KindString:
		s, _ := val.AsStr()
		out, err = formatx.FormatStr(spec, s)
	case tera.KindI64, tera.KindI128, tera.KindU64:
		n, _ := val.AsI128()
		out, err = formatx.FormatInt(spec, n)
	case tera.KindU128:
		n, _ := val.AsU128()
		out, err = formatx.FormatInt(spec, n)
	case tera.KindF64:
		f, _ := val.AsF64()
		out, err = formatx.FormatFloat(spec, f)
	case tera.KindBool:
		b, _ := val.AsBool()
		out, err = formatx.FormatBool(spec, b)
	default:
		return tera.Undefined(), tera.ErrorMessage(fmt.Sprintf("Cannot format value of type %s with format filter",
			val.Name()))
	}
	if err != nil {
		return tera.Undefined(), tera.ErrorMessage("format error: " + err.Error())
	}
	return tera.NormalString(out), nil
})
