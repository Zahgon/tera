package jiff

import (
	"fmt"
	"testing"
)

// TestReferenceTables checks strings obtained from the pinned Jiff 0.2.35 probe.
func TestReferenceTables(t *testing.T) {
	t.Setenv("TZDIR", "/usr/share/zoneinfo")
	tests := []struct {
		op, input, zone, format string
		second                  int64
		nano                    int32
		want                    string
		wantError               bool
	}{
		{"date", "2024-06-01́", "UTC", "", 0, 0, "parsed value '2024-06-01', but unparsed input \"\\u{301}\" remains (expected no unparsed input)", true},
		{"date", "2024-06-01 ", "UTC", "", 0, 0, "parsed value '2024-06-01', but unparsed input \"\\u{a0}\" remains (expected no unparsed input)", true},
		{"date", "2024-06-01🫩", "UTC", "", 0, 0, "parsed value '2024-06-01', but unparsed input \"🫩\" remains (expected no unparsed input)", true},
		{"date", "2024-06-01𐗀", "UTC", "", 0, 0, "parsed value '2024-06-01', but unparsed input \"𐗀\" remains (expected no unparsed input)", true},
		{"date", "", "UTC", "", 0, 0, "expected four digit year (or leading sign for six digit year), but found end of input", true},
		{"date", "2024", "UTC", "", 0, 0, "expected two digit month, but found end of input", true},
		{"date", "2024-06", "UTC", "", 0, 0, "failed to parse separator after month: expected `-` separator, but found end of input", true},
		{"date", "2024/06/01", "UTC", "", 0, 0, "failed to parse two digit integer as month: invalid digit, expected 0-9 but got /", true},
		{"date", "2024-06-1", "UTC", "", 0, 0, "expected two digit day, but found end of input", true},
		{"date", "202406-01", "UTC", "", 0, 0, "failed to parse separator after month: expected no separator since none was found after the year, but found a `-` separator", true},
		{"date", "2024-02-30", "UTC", "", 0, 0, "parsed date is not valid: parameter 'day' for `2024-02` is invalid, must be in range `1..=29`", true},
		{"date", "-000001-02-30", "UTC", "", 0, 0, "parsed date is not valid: parameter 'day' for `-001-02` is invalid, must be in range `1..=28`", true},
		{"date", "-000000-01-01", "UTC", "", 0, 0, "year zero must be written without a sign or a positive sign, but not a negative sign", true},
		{"date", "+010000-01-01", "UTC", "", 0, 0, "failed to parse six digit integer as year: parameter 'year' is not in the required range of -9999..=9999", true},
		{"date", "2024-W22-6", "UTC", "", 0, 0, "failed to parse two digit integer as month: invalid digit, expected 0-9 but got W", true},
		{"date", "2024-153", "UTC", "", 0, 0, "failed to parse two digit integer as month: parameter 'month' is not in the required range of 1..=12", true},
		{"date", "2024-06-01T", "UTC", "", 0, 0, "expected two digit hour, but found end of input", true},
		{"date", "2024-06-01T24:00", "UTC", "", 0, 0, "failed to parse two digit integer as hour: parameter 'hour' is not in the required range of 0..=23", true},
		{"date", "2024-06-01T10:60", "UTC", "", 0, 0, "failed to parse two digit integer as minute: parameter 'minute' is not in the required range of 0..=59", true},
		{"date", "2024-06-01T10:00:61", "UTC", "", 0, 0, "failed to parse two digit integer as second: parameter 'second' is not in the required range of 0..=60", true},
		{"date", "2024-06-01T10:00:00.", "UTC", "", 0, 0, "failed to parse fractional seconds in time: found decimal after seconds component, but did not find any digits after decimal", true},
		{"date", "2024-06-01T10:00:00.1234567890", "UTC", "", 0, 0, "parsed value '2024-06-01T10:00:00.123456789', but unparsed input \"0\" remains (expected no unparsed input)", true},
		{"date", "2024-06-01T10:00:00Z", "UTC", "", 0, 0, "cannot parse civil date/time from string with a Zulu offset, parse as a `jiff::Timestamp` first and convert to a civil date/time instead", true},
		{"date", "2024-06-01T10:00:00+26", "UTC", "", 0, 0, "failed to parse hours in UTC numeric offset: failed to parse hours (requires a two digit integer): parameter 'time zone offset hours' is not in the required range of -25..=25", true},
		{"date", "2024-06-01T10:00:00+05:a0", "UTC", "", 0, 0, "failed to parse minutes in UTC numeric offset: failed to parse minutes (requires a two digit integer): invalid digit, expected 0-9 but got a", true},
		{"date", "2024-06-01T10:00:00+05:30:60", "UTC", "", 0, 0, "failed to parse seconds in UTC numeric offset: failed to parse seconds (requires a two digit integer): parameter 'time zone offset seconds' is not in the required range of -59..=59", true},
		{"date", "2024-06-01T10:00:00+0530:15", "UTC", "", 0, 0, "parsed value '2024-06-01T10:00:00+05:30', but unparsed input \":15\" remains (expected no unparsed input)", true},
		{"date", "2024-06-01[!foo=bar]", "UTC", "", 0, 0, "found unsupported RFC 9557 annotation with the critical flag (`!`) set", true},
		{"date", "2024-06-01[foo=bar][UTC]", "UTC", "", 0, 0, "expected lowercase alphabetic byte (or underscore) at the start of an RFC 9557 annotation key, but found `U` instead", true},
		{"date", "2024-06-01[UTC][Europe/Paris]", "UTC", "", 0, 0, "expected lowercase alphabetic byte (or underscore) at the start of an RFC 9557 annotation key, but found `E` instead", true},
		{"date", "2024-06-01[]", "UTC", "", 0, 0, "expected ASCII alphabetic byte (or underscore or period) at the start of an RFC 9557 annotation or time zone component name, but found `]` instead", true},
		{"date", "2024-06-01[+05:30:00]", "UTC", "", 0, 0, "subminute precision for UTC numeric offset is not enabled in this context (must provide only integral minutes)", true},
		{"date", "2024-06-01[foo=-bar]", "UTC", "", 0, 0, "expected alphanumeric ASCII byte at the start of an RFC 9557 annotation value, but found `-` instead", true},
		{"date", "2024-06-01[UTC", "UTC", "", 0, 0, "expected an `]` after parsing an RFC 9557 time zone annotation, but found end of input instead", true},
		{"date", "2024-06-01\u00e9", "UTC", "", 0, 0, "parsed value '2024-06-01', but unparsed input \"\u00e9\" remains (expected no unparsed input)", true},
		{"date", "2024-06-01T10:00:00Z\n", "UTC", "", 0, 0, "parsed value '2024-06-01T10:00:00Z', but unparsed input \"\\n\" remains (expected no unparsed input)", true},
		{"date", "20240601", "UTC", "", 0, 0, "2024-06-01", false},
		{"date", "+002024-06-01", "UTC", "", 0, 0, "2024-06-01", false},
		{"date", "-000001-06-01", "UTC", "", 0, 0, "-000001-06-01", false},
		{"date", "0000-02-29", "UTC", "", 0, 0, "0000-02-29", false},
		{"date", "2024-06-01T10:30+05:30[Narnia]", "UTC", "", 0, 0, "2024-06-01", false},
		{"date", "2024-06-01[foo=bar-baz]", "UTC", "", 0, 0, "2024-06-01", false},
		{"datetime", "2024-06-01", "UTC", "", 0, 0, "2024-06-01T00:00:00", false},
		{"datetime", "2024-06-01t10", "UTC", "", 0, 0, "2024-06-01T10:00:00", false},
		{"datetime", "20240601 1030", "UTC", "", 0, 0, "2024-06-01T10:30:00", false},
		{"datetime", "2024-06-01T10:30:60,123400000-053015.5[UTC]", "UTC", "", 0, 0, "2024-06-01T10:30:59.1234", false},
		{"datetime", "2024-06-01T10:00:00Z", "UTC", "", 0, 0, "cannot parse civil date/time from string with a Zulu offset, parse as a `jiff::Timestamp` first and convert to a civil date/time instead", true},
		{"timestamp", "2024-06-01", "UTC", "", 0, 0, "failed to find time component, which is required for parsing a timestamp", true},
		{"timestamp", "2024-06-01T10:00", "UTC", "", 0, 0, "failed to find offset component, which is required for parsing a timestamp", true},
		{"timestamp", "2024-06-01T10:00Z", "UTC", "", 0, 0, "2024-06-01T10:00:00Z", false},
		{"timestamp", "2024-06-01T10:00:00.123+05:30", "UTC", "", 0, 0, "2024-06-01T04:30:00.123Z", false},
		{"timestamp", "2024-06-01T10:00:60-00:00:00.5", "UTC", "", 0, 0, "2024-06-01T10:01:00Z", false},
		{"timestamp", "2024-06-01T10:00:00+25:59:59.5", "UTC", "", 0, 0, "due to precision loss from fractional seconds, time zone offset is rounded to a value that is out of bounds", true},
		{"timestamp", "9999-12-31T23:59:59Z", "UTC", "", 0, 0, "failed to convert civil datetime to timestamp with offset +00: converting datetime with time zone offset `+00` to timestamp overflowed: parameter 'Unix timestamp seconds' is not in the required range of -377705023201..=253402207200", true},
		{"pieces", "2024-06-01", "UTC", "", 0, 0, "none", false},
		{"pieces", "2024-06-01T10:00[+05:30]", "UTC", "", 0, 0, "none", false},
		{"pieces", "2024-06-01T10:00Z", "UTC", "", 0, 0, "0", false},
		{"pieces", "2024-06-01T10:00-00:00", "UTC", "", 0, 0, "0", false},
		{"pieces", "2024-06-01T10:00:00+05:30:15.5", "UTC", "", 0, 0, "19816", false},
		{"pieces", "2024-06-01T10:00:00+25:59:59.5", "UTC", "", 0, 0, "due to precision loss from fractional seconds, time zone offset is rounded to a value that is out of bounds", true},
		{"zoned", "2024-06-01", "UTC", "", 0, 0, "failed to find time zone annotation in square brackets, which is required for parsing a zoned datetime", true},
		{"zoned", "2024-06-01[Europe/Paris]", "UTC", "", 0, 0, "2024-06-01T00:00:00+02:00[Europe/Paris]", false},
		{"zoned", "2024-06-01T10:00:00Z[Europe/Paris]", "UTC", "", 0, 0, "2024-06-01T12:00:00+02:00[Europe/Paris]", false},
		{"zoned", "2024-06-01T10:00:00-05[Europe/Paris]", "UTC", "", 0, 0, "datetime could not resolve to a timestamp since `reject` conflict resolution was chosen, and because datetime has offset `-05`, but the time zone `Europe/Paris` for the given datetime unambiguously has offset `+02`", true},
		{"zoned", "2024-06-01T10:00[Narnia]", "UTC", "", 0, 0, "failed to find time zone `Narnia` in time zone database", true},
		{"zoned", "2024-06-01T10:00[-08:00]", "UTC", "", 0, 0, "2024-06-01T10:00:00-08:00[-08:00]", false},
		{"zoned", "2024-06-01T10:00[+00:00]", "UTC", "", 0, 0, "2024-06-01T10:00:00+00:00[UTC]", false},
		{"zoned", "2024-06-01T10:00[Etc/Unknown]", "UTC", "", 0, 0, "2024-06-01T10:00:00Z[Etc/Unknown]", false},
		{"zoned", "2024-03-10T02:30[America/New_York]", "UTC", "", 0, 0, "2024-03-10T03:30:00-04:00[America/New_York]", false},
		{"zoned", "2024-11-03T01:30[America/New_York]", "UTC", "", 0, 0, "2024-11-03T01:30:00-04:00[America/New_York]", false},
		{"zoned", "2024-11-03T01:30-05[America/New_York]", "UTC", "", 0, 0, "2024-11-03T01:30:00-05:00[America/New_York]", false},
		{"zoned", "2024-11-03T01:30-06[America/New_York]", "UTC", "", 0, 0, "datetime could not resolve to timestamp since `reject` conflict resolution was chosen, and because datetime has offset `-06`, but the time zone `America/New_York` for the given datetime falls in a fold between offsets `-04` and `-05`, neither of which match the offset", true},
		{"zoned", "2024-03-10T02:30-05[America/New_York]", "UTC", "", 0, 0, "datetime could not resolve to timestamp since `reject` conflict resolution was chosen, and because datetime has offset `-05`, but the time zone `America/New_York` for the given datetime falls in a gap (between offsets `-05` and `-04`), and all offsets for a gap are regarded as invalid", true},
		{"zoned", "2024-04-07T01:45[Australia/Lord_Howe]", "UTC", "", 0, 0, "2024-04-07T01:45:00+11:00[Australia/Lord_Howe]", false},
		{"zoned", "2024-10-06T02:15[Australia/Lord_Howe]", "UTC", "", 0, 0, "2024-10-06T02:45:00+11:00[Australia/Lord_Howe]", false},
		{"zoned", "1900-03-01T00:30[Europe/Paris]", "UTC", "", 0, 0, "1900-03-01T00:30:00+00:09[Europe/Paris]", false},
		{"zoned", "1900-03-01T00:30+00:09[Europe/Paris]", "UTC", "", 0, 0, "1900-03-01T00:30:00+00:09[Europe/Paris]", false},
		{"zoned", "1900-03-01T00:30+00:09:00[Europe/Paris]", "UTC", "", 0, 0, "datetime could not resolve to a timestamp since `reject` conflict resolution was chosen, and because datetime has offset `+00:09`, but the time zone `Europe/Paris` for the given datetime unambiguously has offset `+00:09:21`", true},
		{"zoned", "2050-06-01T10:00[America/New_York]", "UTC", "", 0, 0, "2050-06-01T10:00:00-04:00[America/New_York]", false},
		{"zoned", "-009999-01-01[UTC]", "UTC", "", 0, 0, "error converting datetime to instant in time zone UTC: converting datetime with time zone offset `+00` to timestamp overflowed: parameter 'Unix timestamp seconds' is not in the required range of -377705023201..=253402207200", true},
		{"dtzone", "2024-03-10T02:30", "America/New_York", "", 0, 0, "2024-03-10T03:30:00-04:00[America/New_York]", false},
		{"dtzone", "2024-11-03T01:30", "America/New_York", "", 0, 0, "2024-11-03T01:30:00-04:00[America/New_York]", false},
		{"dtzone", "2011-12-30T12:30", "Pacific/Apia", "", 0, 0, "2011-12-31T12:30:00+14:00[Pacific/Apia]", false},
		{"dtzone", "2024-10-06T02:15", "Australia/Lord_Howe", "", 0, 0, "2024-10-06T02:45:00+11:00[Australia/Lord_Howe]", false},
		{"dtzone", "9999-12-31T12:00", "UTC", "", 0, 0, "converting datetime with time zone offset `+00` to timestamp overflowed: parameter 'Unix timestamp seconds' is not in the required range of -377705023201..=253402207200", true},
		{"datezone", "2024-03-10T02:30", "America/New_York", "", 0, 0, "2024-03-10T00:00:00-05:00[America/New_York]", false},
		{"datezone", "2024-11-03T01:30", "America/New_York", "", 0, 0, "2024-11-03T00:00:00-04:00[America/New_York]", false},
		{"datezone", "2011-12-30T12:30", "Pacific/Apia", "", 0, 0, "2011-12-31T00:00:00+14:00[Pacific/Apia]", false},
		{"datezone", "2024-10-06T02:15", "Australia/Lord_Howe", "", 0, 0, "2024-10-06T00:00:00+10:30[Australia/Lord_Howe]", false},
		{"datezone", "9999-12-31T12:00", "UTC", "", 0, 0, "converting datetime with time zone offset `+00` to timestamp overflowed: parameter 'Unix timestamp seconds' is not in the required range of -377705023201..=253402207200", true},
		{"new", "", "UTC", "", 0, 0, "1970-01-01T00:00:00Z", false},
		{"new", "", "UTC", "", 0, -1, "1969-12-31T23:59:59.999999999Z", false},
		{"new", "", "UTC", "", -1, 999999999, "1969-12-31T23:59:59.999999999Z", false},
		{"new", "", "UTC", "", 1, -999999999, "1970-01-01T00:00:00.000000001Z", false},
		{"new", "", "UTC", "", -377705023201, 0, "-009999-01-02T01:59:59Z", false},
		{"new", "", "UTC", "", -377705023201, -1, "parameter 'Unix timestamp seconds' is not in the required range of -377705023201..=253402207200", true},
		{"new", "", "UTC", "", 253402207200, 999999999, "9999-12-30T22:00:00.999999999Z", false},
		{"new", "", "UTC", "", 253402207201, 0, "parameter 'Unix timestamp seconds' is not in the required range of -377705023201..=253402207200", true},
		{"new", "", "UTC", "", 9223372036854775807, 0, "parameter 'Unix timestamp seconds' is not in the required range of -377705023201..=253402207200", true},
		{"new", "", "UTC", "", 0, 1000000000, "parameter 'subsecond nanosecond' is not in the required range of -999999999..=999999999", true},
		{"zone", "", "utc", "", 0, 0, "1970-01-01T00:00:00+00:00[UTC]", false},
		{"zone", "", "uTc", "", 0, 0, "1970-01-01T00:00:00+00:00[UTC]", false},
		{"zone", "", "etc/unknown", "", 0, 0, "1970-01-01T00:00:00Z[Etc/Unknown]", false},
		{"zone", "", "america/new_york", "", 0, 0, "1969-12-31T19:00:00-05:00[America/New_York]", false},
		{"zone", "", "Asia/KOLKATA", "", 0, 0, "1970-01-01T05:30:00+05:30[Asia/Kolkata]", false},
		{"zone", "", "EST5EDT", "", 0, 0, "1969-12-31T19:00:00-05:00[EST5EDT]", false},
		{"zone", "", "Narnia", "", 0, 0, "unknown", false},
		{"zone", "", "UTC0", "", 0, 0, "unknown", false},
		{"zone", "", "posix/UTC", "", 0, 0, "unknown", false},
		{"zone", "", "../UTC", "", 0, 0, "unknown", false},
		{"format", "", "America/New_York", "%A|%a|%B|%b|%h|%C|%c", 1717236000, 123400000, "Saturday|Sat|June|Jun|Jun|20|2024 M06 1, Sat 06:00:00", false},
		{"format", "", "America/New_York", "%d|%e|%D|%F|%Y|%y|%j|%q", 1717236000, 123400000, "01| 1|06/01/24|2024-06-01|2024|24|153|2", false},
		{"format", "", "America/New_York", "%H|%I|%k|%l|%M|%S|%p|%P|%r|%R|%T|%X|%x", 1717236000, 123400000, "06|06| 6| 6|00|00|AM|am|6:00:00 AM|06:00|06:00:00|06:00:00|2024 M06 1", false},
		{"format", "", "America/New_York", "%G|%g|%U|%W|%V|%u|%w|%s", 1717236000, 123400000, "2024|24|21|22|22|6|6|1717236000", false},
		{"format", "", "America/New_York", "%f|%3f|%6f|%N|%.f|%.3f|%3.f|%.0f", 1717236000, 123400000, "1234|123|123400|123400000|.1234|.123|.1234|", false},
		{"format", "", "America/New_York", "%z|%:z|%::z|%:::z|%Z|%#Z|%Q|%:Q", 1717236000, 123400000, "-0400|-04:00|-04:00:00|-04|EDT|edt|America/New_York|America/New_York", false},
		{"format", "", "America/New_York", "%_6Y|%-3d|%04e|%^B|%#B|%^c|%#p|%#P", 1717236000, 123400000, "  2024|1|0001|JUNE|June|2024 M06 1, SAT 06:00:00|am|am", false},
		{"format", "", "America/New_York", "%255d|%255s|%255f|%.255f", 1717236000, 123400000, "00000000000000000001|          1717236000|123400000|.123400000", false},
		{"format", "", "America/New_York", "%%|%n|%t", 1717236000, 123400000, "%|\n|\t", false},
		{"format", "", "America/New_York", "%", 1717236000, 123400000, "strftime formatting failed: invalid format string, expected byte after `%`, but found end of format string", true},
		{"format", "", "America/New_York", "%.", 1717236000, 123400000, "strftime formatting failed: invalid format string, expected directive after `%.`", true},
		{"format", "", "America/New_York", "%1", 1717236000, 123400000, "strftime formatting failed: expected to find specifier directive after parsed width, but found end of format string", true},
		{"format", "", "America/New_York", "%+S", 1717236000, 123400000, "strftime formatting failed: found unrecognized specifier directive `+`", true},
		{"format", "", "America/New_York", "%_", 1717236000, 123400000, "strftime formatting failed: expected to find specifier directive after flag `_`, but found end of format string", true},
		{"format", "", "America/New_York", "%256Y", 1717236000, 123400000, "strftime formatting failed: parsed width is too big, max is 255", true},
		{"format", "", "America/New_York", "%999999999999999999999Y", 1717236000, 123400000, "strftime formatting failed: failed to parse conversion specifier width: number too big to parse into 64-bit integer", true},
		{"format", "", "America/New_York", "%.2N", 1717236000, 123400000, "strftime formatting failed: found unrecognized specifier directive `N` following `%.`", true},
		{"format", "", "America/New_York", "%00f", 1717236000, 123400000, "strftime formatting failed: %f failed: zero precision with %f is not allowed", true},
		{"format", "", "America/New_York", "%00N", 1717236000, 123400000, "strftime formatting failed: %N failed: zero precision with %N is not allowed", true},
		{"format", "", "America/New_York", "%::Q", 1717236000, 123400000, "strftime formatting failed: invalid number of `:` in `%Q` directive", true},
		{"format", "", "America/New_York", "%::::z", 1717236000, 123400000, "strftime formatting failed: found unrecognized specifier directive `:`", true},
		{"format", "", "America/New_York", "%^_Y", 1717236000, 123400000, "strftime formatting failed: found unrecognized specifier directive `_`", true},
		{"format", "", "America/New_York", "\u00e9%\u2603", 1717236000, 123400000, "strftime formatting failed: found unrecognized specifier directive `\\xE2`", true},
		{"format", "-000024-06-01T10:00:00[UTC]", "UTC", "%Y|%y|%C|%G|%g|%D|%F|%c|%j", 0, 0, "-024|-24|0|-024|-24|06/01/-24|-0024-06-01|-024 M06 1, Tue 10:00:00|153", false},
		{"format", "-000024-06-01T10:00:00[UTC]", "UTC", "%255Y|%0255Y|%_255Y", 0, 0, "-00000000000000000024|-00000000000000000024|                                                                                                                                                                                                                                                            -24", false},
		{"format", "2024-06-01T10:00[Etc/Unknown]", "UTC", "%Z|%Q|%:Q", 0, 0, "UTC|+0000|+00:00", false},
		{"format", "2024-06-01T10:00[-05:30]", "UTC", "%Z|%z|%Q|%:Q", 0, 0, "-05:30|-0530|-0530|-05:30", false},
		{"format", "2024-06-01T10:00[UTC]", "UTC", "%f|%N|%.f|%.3f", 0, 0, "0|000000000||.000", false},
	}
	for _, tt := range tests {
		t.Run(tt.op+"/"+tt.input+tt.format+tt.zone+fmt.Sprint(tt.second, tt.nano), func(t *testing.T) {
			var got fmt.Stringer
			var err error
			switch tt.op {
			case "zoned":
				got, err = ParseZoned(tt.input)
			case "timestamp":
				got, err = ParseTimestamp(tt.input)
			case "datetime":
				got, err = ParseDateTime(tt.input)
			case "date":
				got, err = ParseDate(tt.input)
			case "new":
				got, err = NewTimestamp(tt.second, tt.nano)
			case "dtzone", "datezone":
				tz, e := GetTimeZone(tt.zone)
				if e != nil {
					t.Fatal(e)
				}
				if tt.op == "dtzone" {
					d, e := ParseDateTime(tt.input)
					err = e
					if e == nil {
						got, err = d.ToZoned(tz)
					}
				} else {
					d, e := ParseDate(tt.input)
					err = e
					if e == nil {
						got, err = d.ToZoned(tz)
					}
				}
			case "pieces":
				p, e := ParsePieces(tt.input)
				err = e
				s := "none"
				if n, ok := p.ToNumericOffset(); ok {
					s = fmt.Sprint(n)
				}
				got = testString(s)
			case "zone":
				tz, e := GetTimeZone(tt.zone)
				if e != nil {
					got = testString("unknown")
				} else {
					ts, e := NewTimestamp(0, 0)
					if e != nil {
						t.Fatal(e)
					}
					got = ts.ToZoned(tz)
				}
			case "format":
				var z Zoned
				if tt.input != "" {
					z, err = ParseZoned(tt.input)
				} else {
					var ts Timestamp
					ts, err = NewTimestamp(tt.second, tt.nano)
					tz, e := GetTimeZone(tt.zone)
					if e != nil {
						t.Fatal(e)
					}
					z = ts.ToZoned(tz)
				}
				if err == nil {
					var s string
					s, err = Format(tt.format, z)
					got = testString(s)
				}
			default:
				t.Fatalf("unrecognized test operation %q", tt.op)
			}
			if err != nil {
				if !tt.wantError || err.Error() != tt.want {
					t.Fatalf("error = %q; want %q (error=%v)", err, tt.want, tt.wantError)
				}
				return
			}
			if tt.wantError {
				t.Fatalf("expected error %q; got %v", tt.want, got)
			}
			if got.String() != tt.want {
				t.Fatalf("got %q; want %q", got, tt.want)
			}
		})
	}
}

type testString string

// String implements fmt.Stringer for textual test results.
func (s testString) String() string { return string(s) }

// TestInstantComparisonAndConversion checks ordering and time zone attachment.
func TestInstantComparisonAndConversion(t *testing.T) {
	a, err := ParseZoned("2024-06-01T10:00:00[Europe/Paris]")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseZoned("2024-06-01T04:00:00[America/New_York]")
	if err != nil {
		t.Fatal(err)
	}
	if a.Compare(b) != 0 {
		t.Fatal("equal instants compare differently")
	}
	c, err := ParseZoned("2024-06-01T08:00:00.000000001[UTC]")
	if err != nil {
		t.Fatal(err)
	}
	if a.Compare(c) != -1 || c.Compare(a) != 1 {
		t.Fatal("nanosecond comparison failed")
	}
	if got, want := a.WithTimeZone(UTC).String(), "2024-06-01T08:00:00+00:00[UTC]"; got != want {
		t.Fatalf("WithTimeZone(UTC) = %q, want %q", got, want)
	}
	ts, err := NewTimestamp(0, -1)
	if err != nil {
		t.Fatal(err)
	}
	z := ts.ToZoned(FixedTimeZone(19815))
	if got, want := z.String(), "1970-01-01T05:30:14.999999999+05:30[+05:30]"; got != want {
		t.Fatalf("ToZoned(+05:30:15) = %q, want %q", got, want)
	}
	const want = "+05:30:15|+053015|+05:30:15|+05:30:15|+05:30:15|0|999999999"
	if got, err := Format("%Z|%z|%:z|%::z|%:::z|%s|%N", z); err != nil || got != want {
		t.Fatalf("Format = %q, %v; want %q", got, err, want)
	}
}
