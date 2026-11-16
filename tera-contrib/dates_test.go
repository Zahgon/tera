package contrib

import (
	"github.com/Keats/tera/tera"
	"testing"
)

func TestOKDate(t *testing.T) {
	// The locale cases require CLDR data and are explicitly excluded from this port.
	for _, tc := range []struct {
		input                  any
		format, timezone, want string
	}{
		{1482720453, "", "", "2016-12-26"}, {1482720453, "%Y-%m-%d %H:%M", "", "2016-12-26 02:47"}, {"1985-04-12T23:20:50.52Z", "", "", "1985-04-12"},
		{"1996-12-19T16:39:57[-08:00]", "%Y-%m-%d %z", "", "1996-12-19 -0800"}, {"1996-12-19T16:39:57-08:00", "%Y-%m-%d %H:%M %z", "", "1996-12-19 16:39 -0800"},
		{"2017-03-05", "%a, %d %b %Y %H:%M:%S %z", "", "Sun, 05 Mar 2017 00:00:00 +0000"}, {"2017-03-05T00:00:00.602", "%a, %d %b %Y %H:%M:%S", "", "Sun, 05 Mar 2017 00:00:00"},
		{"2019-09-19T01:48:44.581Z", "", "America/New_York", "2019-09-18"}, {1648252203, "", "Europe/Berlin", "2022-03-26"},
	} {
		k := map[string]any{}
		if tc.format != "" {
			k["format"] = tc.format
		}
		if tc.timezone != "" {
			k["timezone"] = tc.timezone
		}
		checkFilter(t, Date, tc.input, k, tc.want)
	}
}
func TestBadDateCall(t *testing.T) {
	for _, tc := range []struct {
		input            any
		format, timezone string
	}{{1482720453, "%1", ""}, {1482720453, "%+S", ""}, {"2019-09-19T01:48:44.581Z", "%+S", "Narnia"}} {
		k := map[string]any{"format": tc.format}
		if tc.timezone != "" {
			k["timezone"] = tc.timezone
		}
		checkFilterError(t, Date, tc.input, k)
	}
}
func TestIsBefore(t *testing.T) {
	for _, tc := range []struct {
		input, other    any
		inclusive, want bool
	}{{500, 1000, false, true}, {1000, 500, false, false}, {"2024-01-01", "2024-06-01", false, true}, {"2024-06-01", "2024-01-01", false, false}, {1000, "2020-01-01", false, true}, {"2024-01-01", "2024-01-01", false, false}, {"2024-01-01", "2024-01-01", true, true}} {
		checkDateTest(t, IsBefore, tc.input, tc.other, tc.inclusive, tc.want)
	}
}
func TestIsAfter(t *testing.T) {
	for _, tc := range []struct {
		input, other    any
		inclusive, want bool
	}{{1000, 500, false, true}, {500, 1000, false, false}, {"2024-06-01", "2024-01-01", false, true}, {"2024-01-01", "2024-06-01", false, false}, {"2020-01-01", 1000, false, true}, {"2024-01-01", "2024-01-01", false, false}, {"2024-01-01", "2024-01-01", true, true}} {
		checkDateTest(t, IsAfter, tc.input, tc.other, tc.inclusive, tc.want)
	}
}
func checkDateTest(t testing.TB, f tera.Test, input, other any, inclusive, want bool) {
	t.Helper()
	k := map[string]any{"other": other}
	if inclusive {
		k["inclusive"] = true
	}
	got, err := f.Call(value(t, input), kwargs(t, k), tera.NewState(tera.NewContext()))
	if err != nil || got != want {
		t.Errorf("expected %t, got %t, %v", want, got, err)
	}
}
