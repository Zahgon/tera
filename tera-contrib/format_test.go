package contrib

import (
	"github.com/Keats/tera/tera"
	"strings"
	"testing"
)

func checkFormat(t testing.TB, cases [][2]string) {
	t.Helper()
	for _, tc := range cases {
		engine := tera.New()
		engine.RegisterFilter("format", Format)
		if err := engine.AddRawTemplate("test", tc[0]); err != nil {
			t.Fatal(err)
		}
		got, err := engine.Render("test", tera.NewContext())
		if err != nil || got != tc[1] {
			t.Errorf("%s: expected %q, got %q, %v", tc[0], tc[1], got, err)
		}
	}
}
func TestFloatPrecision(t *testing.T) {
	checkFormat(t, [][2]string{{"{{ 3.14159 | format(spec='.2') }}", "3.14"}, {"{{ 3.14159 | format(spec='.4') }}", "3.1416"}})
}
func TestIntegerPadding(t *testing.T) {
	checkFormat(t, [][2]string{{"{{ 42 | format(spec='05') }}", "00042"}, {"{{ 42 | format(spec='10') }}", "        42"}})
}
func TestStringAlignment(t *testing.T) {
	checkFormat(t, [][2]string{{"{{ 'hi' | format(spec='>5') }}", "   hi"}, {"{{ 'hi' | format(spec='<5') }}", "hi   "}, {"{{ 'hi' | format(spec='^5') }}", " hi  "}})
}
func TestSign(t *testing.T) {
	checkFormat(t, [][2]string{{"{{ 42 | format(spec='+') }}", "+42"}, {"{{ -42 | format(spec='+') }}", "-42"}})
}
func TestBool(t *testing.T) {
	checkFormat(t, [][2]string{{"{{ true | format(spec='>8') }}", "    true"}})
}
func TestCombined(t *testing.T) {
	checkFormat(t, [][2]string{{"{{ 42 | format(spec='>+10') }}", "       +42"}, {"{{ 3.14159 | format(spec='>10.2') }}", "      3.14"}})
}
func TestHugeWidthRejected(t *testing.T) {
	for _, spec := range []string{"4000000000", ".4000000000", ">99999999999999999999"} {
		_, err := Format.Call(tera.I64(1), kwargs(t, map[string]any{"spec": spec}), tera.NewState(tera.NewContext()))
		if err == nil || !strings.Contains(err.Error(), "maximum allowed") {
			t.Errorf("expected maximum allowed error, got %v", err)
		}
	}
}
func TestDollarReferenceRejected(t *testing.T) {
	for _, spec := range []string{"0$", "0$.2", ">0$"} {
		_, err := Format.Call(tera.I64(50000), kwargs(t, map[string]any{"spec": spec}), tera.NewState(tera.NewContext()))
		if err == nil || !strings.Contains(err.Error(), "$") {
			t.Errorf("expected dollar reference error, got %v", err)
		}
	}
}
