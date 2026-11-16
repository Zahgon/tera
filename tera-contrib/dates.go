package contrib

import (
	"fmt"
	"strings"
	"time"

	"github.com/Keats/tera/tera"
	"github.com/Keats/tera/tera-contrib/internal/jiff"
)

// parseToZoned reads a string (an RFC 3339 or Temporal datetime, a timestamp with an offset, a civil datetime or a
// date) or an integer (seconds since the Unix epoch) as a zoned datetime. Values without a zone or offset are placed
// in tz, or in UTC when tz is nil. For a string that matches none of the forms, the error of the last form tried
// (a date) is reported.
func parseToZoned(val tera.Value, tz *jiff.TimeZone) (jiff.Zoned, error) {
	defaultTZ := jiff.UTC
	if tz != nil {
		defaultTZ = *tz
	}
	if s, ok := val.AsStr(); ok {
		if z, err := jiff.ParseZoned(s); err == nil {
			return z, nil
		}
		if t, err := jiff.ParseTimestamp(s); err == nil {
			zone := defaultTZ
			if p, err := jiff.ParsePieces(s); err == nil {
				if offset, ok := p.ToNumericOffset(); ok {
					zone = jiff.FixedTimeZone(offset)
				}
			}
			return t.ToZoned(zone), nil
		}
		if dt, err := jiff.ParseDateTime(s); err == nil {
			if z, err := dt.ToZoned(defaultTZ); err == nil {
				return z, nil
			}
		}
		d, err := jiff.ParseDate(s)
		if err == nil {
			var z jiff.Zoned
			if z, err = d.ToZoned(defaultTZ); err == nil {
				return z, nil
			}
		}
		return jiff.Zoned{}, tera.ErrorMessage(fmt.Sprintf("The string %s cannot be parsed as a valid date: %s", s, err))
	}
	if n, ok := val.AsNumber(); ok {
		if i, ok := n.AsInteger(); ok {
			if !i.IsInt64() {
				return jiff.Zoned{}, tera.ErrorMessage(fmt.Sprintf("Invalid timestamp: %s", i))
			}
			ts, err := jiff.NewTimestamp(i.Int64(), 0)
			if err != nil {
				return jiff.Zoned{}, tera.ErrorMessage(fmt.Sprintf("Invalid timestamp: %s", err))
			}
			return ts.ToZoned(defaultTZ), nil
		}
	}
	return jiff.Zoned{}, tera.ErrorMessage(fmt.Sprintf("Invalid value: expected a string or integer, got %s",
		val.Name()))
}

// timezoneArg reads the optional `timezone` argument.
func timezoneArg(kwargs tera.Kwargs) (*jiff.TimeZone, error) {
	name, ok, err := tera.KwargsGet(kwargs, "timezone", tera.ArgStr)
	if err != nil || !ok {
		return nil, err
	}
	tz, err := jiff.GetTimeZone(name)
	if err != nil {
		return nil, tera.ErrorMessage("Unknown timezone: " + name)
	}
	return &tz, nil
}

// Now returns the current datetime in the optional `timezone` (UTC by default).
//
//	{{ now() }}
//	{{ now(timezone="America/New_York") }}
var Now = tera.FunctionFunc(func(kwargs tera.Kwargs, _ *tera.State) (tera.Value, error) {
	name, ok, err := tera.KwargsGet(kwargs, "timezone", tera.ArgStr)
	if err != nil {
		return tera.Undefined(), err
	}
	if !ok {
		name = "UTC"
	}
	tz, err := jiff.GetTimeZone(name)
	if err != nil {
		return tera.Undefined(), tera.ErrorMessage("Unknown timezone: " + name)
	}
	now := time.Now()
	ts, err := jiff.NewTimestamp(now.Unix(), int32(now.Nanosecond()))
	if err != nil {
		return tera.Undefined(), tera.ErrorMessage(err.Error())
	}
	return tera.NormalString(ts.ToZoned(tz).String()), nil
})

// Date formats a string or integer that can be read as a date. It takes an optional `format` (a strftime-like
// format, `%Y-%m-%d` by default), an optional `timezone` to convert to, and an optional `locale`, which requires a
// UTS-35 `format`.
//
//	{{ value | date(format="%B %d, %Y") }}
//	{{ timestamp | date(format="%Y-%m-%d %H:%M", timezone="Europe/Paris") }}
var Date = tera.TypedFilter(tera.ArgValueRef, func(val tera.Value, kwargs tera.Kwargs, _ *tera.State) (tera.Value, error) {
	format, hasFormat, err := tera.KwargsGet(kwargs, "format", tera.ArgStr)
	if err != nil {
		return tera.Undefined(), err
	}
	locale, hasLocale, err := tera.KwargsGet(kwargs, "locale", tera.ArgStr)
	if err != nil {
		return tera.Undefined(), err
	}
	tz, err := timezoneArg(kwargs)
	if err != nil {
		return tera.Undefined(), err
	}
	zoned, err := parseToZoned(val, tz)
	if err != nil {
		return tera.Undefined(), err
	}
	if tz != nil {
		zoned = zoned.WithTimeZone(*tz)
	}
	if hasLocale {
		return tera.Undefined(), localizedDate(format, hasFormat, locale)
	}
	if !hasFormat {
		format = "%Y-%m-%d"
	}
	out, err := jiff.Format(format, zoned)
	if err != nil {
		return tera.Undefined(), tera.ErrorMessage(fmt.Sprintf("Invalid date format `%s`: %s", format, err))
	}
	return tera.NormalString(out), nil
})

// localizedDate reports why a localized date cannot be formatted: the arguments are checked as the original
// checks them, and the CLDR locale data that a UTS-35 pattern needs is not part of this module.
func localizedDate(format string, hasFormat bool, locale string) error {
	if !hasFormat {
		return tera.ErrorMessage("Setting `locale` requires setting a `format` as well")
	}
	if strings.Contains(format, "%") {
		return tera.ErrorMessage(fmt.Sprintf("Invalid date format `%s`: strftime formats cannot be localized, "+
			"use a UTS-35 pattern instead (eg `d MMMM y`)", format))
	}
	return tera.ErrorMessage(fmt.Sprintf("Failed to load data for locale %s: no CLDR locale data is available",
		locale))
}

// compareDates parses the value and the mandatory `other` argument as dates and compares them.
func compareDates(val tera.Value, kwargs tera.Kwargs) (int, bool, error) {
	other, err := tera.KwargsMustGet(kwargs, "other", tera.ArgValueRef)
	if err != nil {
		return 0, false, err
	}
	inclusive, _, err := tera.KwargsGet(kwargs, "inclusive", tera.ArgBool)
	if err != nil {
		return 0, false, err
	}
	a, err := parseToZoned(val, nil)
	if err != nil {
		return 0, false, err
	}
	b, err := parseToZoned(other, nil)
	if err != nil {
		return 0, false, err
	}
	return a.Compare(b), inclusive, nil
}

// IsBefore is the `before` test: true when the value is a date before the `other` argument, or equal to it with
// `inclusive=true`. Both must parse as dates.
//
//	{% if date is before(other="2024-06-01") %}...{% endif %}
var IsBefore = tera.TypedTest(tera.ArgValueRef, func(val tera.Value, kwargs tera.Kwargs, _ *tera.State) (bool, error) {
	c, inclusive, err := compareDates(val, kwargs)
	return c < 0 || (inclusive && c == 0), err
})

// IsAfter is the `after` test: true when the value is a date after the `other` argument, or equal to it with
// `inclusive=true`. Both must parse as dates.
//
//	{% if date is after(other="2024-01-01") %}...{% endif %}
var IsAfter = tera.TypedTest(tera.ArgValueRef, func(val tera.Value, kwargs tera.Kwargs, _ *tera.State) (bool, error) {
	c, inclusive, err := compareDates(val, kwargs)
	return c > 0 || (inclusive && c == 0), err
})
