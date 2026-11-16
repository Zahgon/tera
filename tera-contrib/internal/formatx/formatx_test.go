// Reference vectors recorded from the original crate at its locked version (a sample of the full differential set).
package formatx

import (
	"math/big"
	"strconv"
	"testing"
)

func TestRustVectors(t *testing.T) {
	cases := []struct{ spec, kind, arg, want, err string }{
		{"", "str", "", "", ""},
		{"", "str", "hi", "hi", ""},
		{"", "str", "é中🦀", "é中🦀", ""},
		{"", "str", "\n\t\u0000\"\\", "\n\t\u0000\"\\", ""},
		{"", "str", "á", "á", ""},
		{"", "str", "​́a", "​́a", ""},
		{"", "str", "Ᲊ", "Ᲊ", ""},
		{"", "str", "𞗐", "𞗐", ""},
		{"", "bool", "true", "true", ""},
		{"", "bool", "false", "false", ""},
		{"", "int", "0", "0", ""},
		{"", "int", "1", "1", ""},
		{"", "int", "-42", "-42", ""},
		{"", "int", "9223372036854775807", "9223372036854775807", ""},
		{"", "int", "18446744073709551615", "18446744073709551615", ""},
		{"", "int", "-170141183460469231731687303715884105728", "-170141183460469231731687303715884105728", ""},
		{"", "int", "170141183460469231731687303715884105727", "170141183460469231731687303715884105727", ""},
		{"", "uint", "340282366920938463463374607431768211455", "340282366920938463463374607431768211455", ""},
		{"", "float", "0.0", "0", ""},
		{"", "float", "-0.0", "-0", ""},
		{"", "float", "1e-07", "0.0000001", ""},
		{"", "float", "0.1", "0.1", ""},
		{"", "float", "0.5", "0.5", ""},
		{"", "float", "2.5", "2.5", ""},
		{"", "float", "1e+16", "10000000000000000", ""},
		{"", "float", "inf", "inf", ""},
		{"", "float", "1.25", "1.25", ""},
		{"", "float", "0.0001", "0.0001", ""},
		{"?", "str", "", "\"\"", ""},
		{"?", "str", "\n\t\u0000\"\\", "\"\\n\\t\\0\\\"\\\\\"", ""},
		{"x?", "str", "", "\"\"", ""},
		{"X?", "str", "", "\"\"", ""},
		{"e", "str", "", "", "unsupported format trait: `LowerExp`"},
		{"E", "str", "", "", "unsupported format trait: `UpperExp`"},
		{"b", "str", "", "", "unsupported format trait: `Binary`"},
		{"o", "str", "", "", "unsupported format trait: `Octal`"},
		{"x", "str", "", "", "unsupported format trait: `LowerHex`"},
		{"p", "str", "", "", "unsupported format trait: `Pointer`"},
		{".", "str", "", "", "parse error at byte 2: expected precision value after `.`"},
		{".*", "str", "", "", "parse error at byte 0: `.*` precision argument `` is not a valid usize"},
		{"x??", "str", "", "", "parse error at byte 0: unmatched `{`"},
		{"foo", "str", "", "", "parse error at byte 2: unknown format type: `f`"},
		{"+x", "str", "", "", "unsupported format trait: `LowerHex`"},
		{"2.3z", "str", "", "", "parse error at byte 5: unknown format type: `z`"},
		{"é", "str", "", "", "parse error at byte 2: unknown format type: `Ã`"},
		{">", "str", "", "", ""},
		{"}}", "str", "", "}", ""},
		{"}}}", "str", "", "", "parse error at byte 5: unmatched `}`"},
		{"}<5", "str", "", "", "parse error at byte 5: unmatched `}`"},
		{"{<5", "str", "", "{{{{{", ""},
		{"}abc{{", "str", "", "", "parse error at byte 8: unmatched `}`"},
		{"}abc{0", "str", "", "abc", ""},
		{"} {1", "str", "", "", "missing argument: `1`"},
		{"} {name", "str", "", "", "missing argument: `name`"},
		{"} {00", "str", "", " ", ""},
		{"🦀^10.2", "str", "", "🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀", ""},
		{"<+1.5X?", "float", "1e-05", "1e-5", ""},
		{"<+#.1?", "float", "0.0", "+0.0", ""},
		{">-5.0?", "float", "inf", "  inf", ""},
		{"^.100x?", "str", "\n\t\u0000\"\\", "\"\\n\\t\\0\\\"\\\\\"", ""},
		{"0^+#05", "str", "", "00000", ""},
		{">+10.5", "int", "0", "        +0", ""},
		{"🦀>+010?", "str", "é中🦀", "🦀🦀🦀🦀🦀\"é中🦀\"", ""},
		{">#10.20e", "str", "", "", "unsupported format trait: `LowerExp`"},
		{"🦀>+0100.100e", "float", "inf", "", "unsupported format trait: `LowerExp`"},
		{"🦀>+010.2", "float", "2.675", "🦀🦀🦀🦀🦀+2.67", ""},
		{"^-.0", "float", "1e-05", "0", ""},
		{">+#10.2?", "float", "nan", "       NaN", ""},
		{"^-#01.2?", "float", "2.5", "2.50", ""},
		{"0^#.20e", "float", "2.675", "", "unsupported format trait: `LowerExp`"},
		{"<01.20e", "float", "-inf", "", "unsupported format trait: `LowerExp`"},
		{"^-0.20e", "str", "\n\t\u0000\"\\", "", "unsupported format trait: `LowerExp`"},
		{"^+#1", "uint", "340282366920938463463374607431768211455", "+340282366920938463463374607431768211455", ""},
		{"🦀>#0100.20", "float", "1.35", "🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀🦀1.35000000000000008882", ""},
		{"0^05.5", "str", "𞗐", "00𞗐00", ""},
		{"🦀>#100.1e", "float", "1e+21", "", "unsupported format trait: `LowerExp`"},
		{"+#.2", "float", "0.0", "+0.00", ""},
		{"🦀>#010.5X?", "uint", "340282366920938463463374607431768211455", "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", ""},
		{"+#0100.20x?", "float", "1.25", "1.25", ""},
		{">-#5.5", "float", "0.0", "0.00000", ""},
		{">+#10.100", "bool", "true", "      true", ""},
		{">-10.2e", "float", "1.25", "", "unsupported format trait: `LowerExp`"},
		{"<-#0100.20e", "str", "é中🦀", "", "unsupported format trait: `LowerExp`"},
		{"0^-0.20?", "float", "1e+16", "10000000000000000.00000000000000000000", ""},
		{"0^-#.0x?", "int", "1", "1", ""},
		{"<-#x?", "float", "2.5", "2.5", ""},
		{"-#0100.100x?", "float", "1e-07", "1e-7", ""},
		{"^+10.2e", "float", "1.7976931348623157e+308", "", "unsupported format trait: `LowerExp`"},
		{"^-#010.1", "float", "-0.0", "   -0.0   ", ""},
		{"-#5.20x?", "float", "2.675", "2.675", ""},
		{"<01.1e", "float", "5e-324", "", "unsupported format trait: `LowerExp`"},
		{"+05.100e", "str", "𞗐", "", "unsupported format trait: `LowerExp`"},
		{"+01.0", "bool", "true", " ", ""},
		{"-10.0", "float", "0.1", "         0", ""},
		{"^+05.5?", "str", "Ᲊ", " \"Ᲊ\" ", ""},
	}
	for i, c := range cases {
		var got string
		var err error
		switch c.kind {
		case "str":
			got, err = FormatStr(c.spec, c.arg)
		case "bool":
			b, _ := strconv.ParseBool(c.arg)
			got, err = FormatBool(c.spec, b)
		case "int", "uint":
			n, _ := new(big.Int).SetString(c.arg, 10)
			got, err = FormatInt(c.spec, n)
		default:
			f, _ := strconv.ParseFloat(c.arg, 64)
			got, err = FormatFloat(c.spec, f)
		}
		message := ""
		if err != nil {
			message = err.Error()
		}
		if got != c.want || message != c.err {
			t.Errorf("case %d spec %q kind %s: %q %q, want %q %q", i, c.spec, c.kind, got, message, c.want, c.err)
		}
	}
}
