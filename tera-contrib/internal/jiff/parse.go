package jiff

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const civilZulu = "cannot parse civil date/time from string with a Zulu offset, parse as a `jiff::Timestamp` first and convert to a civil date/time instead"
const precisionLoss = "due to precision loss from fractional seconds, time zone offset is rounded to a value that is out of bounds"

// Pieces contains the parsed components of a Temporal datetime string.
type Pieces struct{ p parsed }

// ToNumericOffset returns the explicit UTC offset, including zero for Z.
func (p Pieces) ToNumericOffset() (offsetSeconds int32, ok bool) {
	return int32(p.p.off.seconds()), p.p.hasOffset
}

// ParsePieces parses a Temporal datetime into its components.
func ParsePieces(s string) (Pieces, error) {
	p, err := parse(s)
	if err == nil && p.hasOffset {
		err = p.off.validate()
	}
	return Pieces{p}, err
}

// ParseDate parses a Temporal string and extracts its civil calendar date.
func ParseDate(s string) (Date, error) {
	p, err := parse(s)
	if err != nil {
		return Date{}, err
	}
	if p.off.zulu {
		return Date{}, errors.New(civilZulu)
	}
	return Date{time.Date(p.dt.Year(), p.dt.Month(), p.dt.Day(), 0, 0, 0, 0, time.UTC)}, nil
}

// ParseDateTime parses a Temporal string and extracts its civil datetime.
func ParseDateTime(s string) (DateTime, error) {
	p, err := parse(s)
	if err != nil {
		return DateTime{}, err
	}
	if p.off.zulu {
		return DateTime{}, errors.New(civilZulu)
	}
	return DateTime{p.dt}, nil
}

// ParseTimestamp parses a Temporal datetime with an explicit UTC offset.
func ParseTimestamp(s string) (Timestamp, error) {
	p, err := parse(s)
	if err != nil {
		return Timestamp{}, err
	}
	return p.timestamp()
}

func (p parsed) timestamp() (Timestamp, error) {
	if !p.hasTime {
		return Timestamp{}, errors.New("failed to find time component, which is required for parsing a timestamp")
	}
	if !p.hasOffset {
		return Timestamp{}, errors.New("failed to find offset component, which is required for parsing a timestamp")
	}
	if err := p.off.validate(); err != nil {
		return Timestamp{}, err
	}
	sec := p.dt.Unix() - int64(p.off.seconds())
	t, err := NewTimestamp(sec, int32(p.dt.Nanosecond()))
	if err != nil {
		off := offsetString(p.off.seconds(), true, false, false)
		return Timestamp{}, fmt.Errorf("failed to convert civil datetime to timestamp with offset %s: converting datetime with time zone offset `%s` to timestamp overflowed: %s", off, off, err)
	}
	return t, nil
}

// ParseZoned parses a Temporal datetime with a bracketed time zone, rejecting
// explicit numeric offsets that conflict with the time zone's rules.
func ParseZoned(s string) (Zoned, error) {
	ensureZoneDatabase()
	p, err := parse(s)
	if err != nil {
		return Zoned{}, err
	}
	if p.zone == "" && !p.zoneFixed {
		return Zoned{}, errors.New("failed to find time zone annotation in square brackets, which is required for parsing a zoned datetime")
	}
	var tz TimeZone
	if p.zoneFixed {
		tz = FixedTimeZone(int32(p.zoneOff.seconds()))
	} else {
		tz, err = GetTimeZone(p.zone)
		if err != nil {
			return Zoned{}, err
		}
	}
	dt := DateTime{p.dt}
	if !p.hasOffset {
		amb := tz.ambiguous(dt.t)
		z, err := civilToZoned(dt.t, amb.before, tz)
		return parsedZonedResult(z, err, tz)
	}
	if p.off.zulu {
		z, err := civilToZoned(p.dt, 0, tz)
		return parsedZonedResult(z, err, tz)
	}
	if err := p.off.validate(); err != nil {
		return Zoned{}, err
	}
	given := p.off.seconds()
	amb := tz.ambiguous(dt.t)
	equal := func(off int) bool {
		if given == off {
			return true
		}
		if p.off.hasSecond || off%60 == 0 {
			return false
		}
		a := off
		if a < 0 {
			a = -a
		}
		rounded := ((a + 30) / 60) * 60
		if rounded > 93599 {
			return false
		}
		if off < 0 {
			rounded = -rounded
		}
		return given == rounded
	}
	if amb.kind == 0 && equal(amb.before) || amb.kind == 'f' && (equal(amb.before) || equal(amb.after)) {
		offset := amb.before
		if amb.kind == 'f' {
			offset = given
		}
		z, err := civilToZoned(dt.t, offset, tz)
		return parsedZonedResult(z, err, tz)
	}

	name := tz.Name()
	if name == "" {
		_, o := p.dt.In(tz.location()).Zone()
		name = offsetString(o, true, false, false)
	}
	gs := offsetString(given, true, false, false)
	if amb.kind == 0 {
		off := amb.before
		return Zoned{}, fmt.Errorf("datetime could not resolve to a timestamp since `reject` conflict resolution was chosen, and because datetime has offset `%s`, but the time zone `%s` for the given datetime unambiguously has offset `%s`", gs, name, offsetString(off, true, false, false))
	}
	before, after := amb.before, amb.after
	if amb.kind == 'f' {
		return Zoned{}, fmt.Errorf("datetime could not resolve to timestamp since `reject` conflict resolution was chosen, and because datetime has offset `%s`, but the time zone `%s` for the given datetime falls in a fold between offsets `%s` and `%s`, neither of which match the offset", gs, name, offsetString(before, true, false, false), offsetString(after, true, false, false))
	}
	return Zoned{}, fmt.Errorf("datetime could not resolve to timestamp since `reject` conflict resolution was chosen, and because datetime has offset `%s`, but the time zone `%s` for the given datetime falls in a gap (between offsets `%s` and `%s`), and all offsets for a gap are regarded as invalid", gs, name, offsetString(before, true, false, false), offsetString(after, true, false, false))
}

type parsed struct {
	dt                 time.Time
	hasTime, hasOffset bool
	off                parsedOffset
	zone               string
	zoneFixed          bool
	zoneOff            parsedOffset
	critical           bool
}
type parsedOffset struct {
	negative, zulu                bool
	hour, minute, second, nano    int
	hasMinute, hasSecond, hasNano bool
}

func (o parsedOffset) seconds() int {
	n := o.hour*3600 + o.minute*60 + o.second
	if o.nano >= 500000000 {
		n++
	}
	if o.negative {
		n = -n
	}
	return n
}
func (o parsedOffset) validate() error {
	if o.seconds() < -93599 || o.seconds() > 93599 {
		return errors.New(precisionLoss)
	}
	return nil
}

// String preserves the precision and sign of a parsed UTC offset.
func (o parsedOffset) String() string {
	if o.zulu {
		return "Z"
	}
	s := "+"
	if o.negative {
		s = "-"
	}
	s += fmt.Sprintf("%02d", o.hour)
	if o.hasMinute {
		s += fmt.Sprintf(":%02d", o.minute)
	}
	if o.hasSecond {
		s += fmt.Sprintf(":%02d", o.second)
	}
	if o.hasNano && o.nano != 0 {
		s += "." + strings.TrimRight(fmt.Sprintf("%09d", o.nano), "0")
	}
	return s
}

// String normalizes parsed components for unconsumed-input diagnostics.
func (p parsed) String() string {
	s := civilString(p.dt, p.hasTime)
	if p.hasOffset {
		s += p.off.String()
	}
	if p.zone != "" || p.zoneFixed {
		s += "["
		if p.critical {
			s += "!"
		}
		if p.zoneFixed {
			s += p.zoneOff.String()
		} else {
			s += p.zone
		}
		s += "]"
	}
	return s
}

type parser struct {
	s string
	i int
}

func (p *parser) rest() string { return p.s[p.i:] }
func (p *parser) take(b byte) bool {
	if p.i < len(p.s) && p.s[p.i] == b {
		p.i++
		return true
	}
	return false
}
func (p *parser) digits(n int, unit, word, param string, min, max int) (int, error) {
	if len(p.rest()) < n {
		return 0, fmt.Errorf("expected %s digit %s, but found end of input", word, unit)
	}
	s := p.s[p.i : p.i+n]
	p.i += n
	v := 0
	for i := 0; i < len(s); i++ {
		if !digit(s[i]) {
			return 0, fmt.Errorf("failed to parse %s digit integer as %s: invalid digit, expected 0-9 but got %s", word, unit, escapeByte(s[i]))
		}
		v = v*10 + int(s[i]-'0')
	}
	if v < min || v > max {
		return 0, fmt.Errorf("failed to parse %s digit integer as %s: parameter '%s' is not in the required range of %d..=%d", word, unit, param, min, max)
	}
	return v, nil
}
func (p *parser) dateSeparator(extended bool, after string) error {
	var err error
	if !extended {
		if strings.HasPrefix(p.rest(), "-") {
			err = errors.New("expected no separator since none was found after the year, but found a `-` separator")
		}
	} else if p.i == len(p.s) {
		err = errors.New("expected `-` separator, but found end of input")
	} else if !p.take('-') {
		err = fmt.Errorf("expected `-` separator, but found `%s`", escapeByte(p.s[p.i]))
	}
	if err != nil {
		return fmt.Errorf("failed to parse separator after %s: %s", after, err)
	}
	return nil
}
func (p *parser) timeSeparator(extended bool) bool {
	if extended {
		return p.take(':')
	}
	s := p.rest()
	return len(s) >= 2 && digit(s[0]) && digit(s[1])
}
func (p *parser) fraction() (int, bool, error) {
	if !p.take('.') && !p.take(',') {
		return 0, false, nil
	}
	n := 0
	count := 0
	for count < 9 && p.i < len(p.s) && digit(p.s[p.i]) {
		n = n*10 + int(p.s[p.i]-'0')
		count++
		p.i++
	}
	if count == 0 {
		return 0, true, errors.New("found decimal after seconds component, but did not find any digits after decimal")
	}
	for count < 9 {
		n *= 10
		count++
	}
	return n, true, nil
}
func parse(s string) (parsed, error) {
	p := parser{s: s}
	r := parsed{}
	sign := 1
	signed := false
	if p.take('+') {
		signed = true
	} else if p.take('-') {
		signed = true
		sign = -1
	}
	n, w := 4, "four"
	if signed {
		n, w = 6, "six"
	}
	if len(p.rest()) < n {
		if signed {
			return r, errors.New("expected six digit year (because of a leading sign), but found end of input")
		}
		return r, errors.New("expected four digit year (or leading sign for six digit year), but found end of input")
	}
	y, err := p.digits(n, "year", w, "year", -9999, 9999)
	if err != nil {
		return r, err
	}
	if y == 0 && sign < 0 {
		return r, errors.New("year zero must be written without a sign or a positive sign, but not a negative sign")
	}
	y *= sign
	extended := strings.HasPrefix(p.rest(), "-")
	if err = p.dateSeparator(extended, "year"); err != nil {
		return r, err
	}
	m, err := p.digits(2, "month", "two", "month", 1, 12)
	if err != nil {
		return r, err
	}
	if err = p.dateSeparator(extended, "month"); err != nil {
		return r, err
	}
	d, err := p.digits(2, "day", "two", "day", 1, 31)
	if err != nil {
		return r, err
	}
	days := time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if d > days {
		ys := fmt.Sprintf("%04d", y)
		return r, fmt.Errorf("parsed date is not valid: parameter 'day' for `%s-%02d` is invalid, must be in range `1..=%d`", ys, m, days)
	}
	h, mi, se, ns := 0, 0, 0, 0
	if p.take('T') || p.take('t') || p.take(' ') {
		r.hasTime = true
		h, err = p.digits(2, "hour", "two", "hour", 0, 23)
		if err != nil {
			return r, err
		}
		ex := strings.HasPrefix(p.rest(), ":")
		if p.timeSeparator(ex) {
			mi, err = p.digits(2, "minute", "two", "minute", 0, 59)
			if err != nil {
				return r, err
			}
			if p.timeSeparator(ex) {
				se, err = p.digits(2, "second", "two", "second", 0, 60)
				if err != nil {
					return r, err
				}
				if se == 60 {
					se = 59
				}
				ns, _, err = p.fraction()
				if err != nil {
					return r, fmt.Errorf("failed to parse fractional seconds in time: %s", err)
				}
			}
		}
		if len(p.rest()) > 0 && strings.ContainsRune("Zz+-", rune(p.rest()[0])) {
			r.hasOffset = true
			r.off, err = p.offset(true)
			if err != nil {
				return r, err
			}
		}
	}
	r.dt = time.Date(y, time.Month(m), d, h, mi, se, ns, time.UTC)
	if strings.HasPrefix(p.rest(), "[") {
		if err = p.annotations(&r); err != nil {
			return r, err
		}
	}
	if p.i < len(s) {
		return r, fmt.Errorf("parsed value '%s', but unparsed input \"%s\" remains (expected no unparsed input)", r.String(), escapeBytes(p.rest()))
	}
	return r, nil
}

func (p *parser) offset(subminute bool) (parsedOffset, error) {
	o := parsedOffset{}
	if p.take('Z') || p.take('z') {
		o.zulu = true
		return o, nil
	}
	o.negative = p.take('-')
	if !o.negative {
		p.take('+')
	}
	extended := false
	for part := 0; part < 3; part++ {
		unit := []string{"hours", "minutes", "seconds"}[part]
		prev := []string{"sign", "hours", "minutes"}[part]
		max := 59
		if part == 0 {
			max = 25
		}
		if len(p.rest()) < 2 {
			return o, fmt.Errorf("failed to parse %s in UTC numeric offset: expected two digit %s after %s, but found end of input", unit, strings.TrimSuffix(unit, "s"), prev)
		}
		v := 0
		for j := 0; j < 2; j++ {
			b := p.s[p.i+j]
			if !digit(b) {
				return o, fmt.Errorf("failed to parse %s in UTC numeric offset: failed to parse %s (requires a two digit integer): invalid digit, expected 0-9 but got %s", unit, unit, escapeByte(b))
			}
			v = v*10 + int(b-'0')
		}
		p.i += 2
		if v > max {
			return o, fmt.Errorf("failed to parse %s in UTC numeric offset: failed to parse %s (requires a two digit integer): parameter 'time zone offset %s' is not in the required range of -%d..=%d", unit, unit, unit, max, max)
		}
		switch part {
		case 0:
			o.hour = v
		case 1:
			o.minute = v
			o.hasMinute = true
		case 2:
			o.second = v
			o.hasSecond = true
		}
		if part == 0 {
			extended = strings.HasPrefix(p.rest(), ":")
			if !p.timeSeparator(extended) {
				return o, nil
			}
		}
		if part == 1 {
			if !subminute {
				if strings.HasPrefix(p.rest(), ":") {
					return o, errors.New("subminute precision for UTC numeric offset is not enabled in this context (must provide only integral minutes)")
				}
				return o, nil
			}
			if !p.timeSeparator(extended) {
				return o, nil
			}
		}
	}
	var err error
	o.nano, o.hasNano, err = p.fraction()
	if err != nil {
		return o, fmt.Errorf("failed to parse fractional seconds in UTC numeric offset: %s", err)
	}
	return o, nil
}

func digit(b byte) bool   { return b >= '0' && b <= '9' }
func alpha(b byte) bool   { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
func tzLead(b byte) bool  { return alpha(b) || b == '_' || b == '.' }
func tzChar(b byte) bool  { return tzLead(b) || digit(b) || b == '-' || b == '+' }
func keyLead(b byte) bool { return b >= 'a' && b <= 'z' || b == '_' }

func (p *parser) annotationExpected(kind string) error {
	prefix := ""
	switch kind {
	case "tzlead":
		prefix = "expected the start of an RFC 9557 annotation or IANA time zone component name"
	case "key":
		prefix = "expected the start of an RFC 9557 annotation key"
	case "value":
		prefix = "expected the start of an RFC 9557 annotation value"
	case "sep":
		prefix = "expected an `=` after parsing an RFC 9557 annotation key"
	case "close":
		prefix = "expected an `]` after parsing an RFC 9557 annotation key and value"
	case "tzclose":
		prefix = "expected an `]` after parsing an RFC 9557 time zone annotation"
	}
	if p.i == len(p.s) {
		return errors.New(prefix + ", but found end of input instead")
	}
	if kind == "tzlead" {
		prefix = "expected ASCII alphabetic byte (or underscore or period) at the start of an RFC 9557 annotation or time zone component name"
	}
	if kind == "key" {
		prefix = "expected lowercase alphabetic byte (or underscore) at the start of an RFC 9557 annotation key"
	}
	if kind == "value" {
		prefix = "expected alphanumeric ASCII byte at the start of an RFC 9557 annotation value"
	}
	if kind == "sep" && p.s[p.i] == '/' {
		return errors.New(prefix + ", but found `/` instead (time zone annotations must come first)")
	}
	return fmt.Errorf("%s, but found `%s` instead", prefix, escapeByte(p.s[p.i]))
}
func (p *parser) tzComponent() error {
	if p.i == len(p.s) || !tzLead(p.s[p.i]) {
		return p.annotationExpected("tzlead")
	}
	p.i++
	for p.i < len(p.s) && tzChar(p.s[p.i]) {
		p.i++
	}
	return nil
}
func (p *parser) annotations(r *parsed) error {
	start := p.i
	p.i++
	critical := p.take('!')
	if strings.HasPrefix(p.rest(), "+") || strings.HasPrefix(p.rest(), "-") {
		o, err := p.offset(false)
		if err != nil {
			return err
		}
		if !p.take(']') {
			return p.annotationExpected("tzclose")
		}
		r.zoneFixed = true
		r.zoneOff = o
		r.critical = critical
	} else {
		nameStart := p.i
		if err := p.tzComponent(); err != nil {
			return err
		}
		if strings.HasPrefix(p.rest(), "=") {
			p.i = start
		} else {
			for p.take('/') {
				if err := p.tzComponent(); err != nil {
					return err
				}
			}
			r.zone = p.s[nameStart:p.i]
			r.critical = critical
			if !p.take(']') {
				return p.annotationExpected("tzclose")
			}
		}
	}
	for p.take('[') {
		critical := p.take('!')
		if p.i == len(p.s) || !keyLead(p.s[p.i]) {
			return p.annotationExpected("key")
		}
		p.i++
		for p.i < len(p.s) && (keyLead(p.s[p.i]) || digit(p.s[p.i]) || p.s[p.i] == '-') {
			p.i++
		}
		if !p.take('=') {
			return p.annotationExpected("sep")
		}
		for {
			if p.i == len(p.s) || !alpha(p.s[p.i]) && !digit(p.s[p.i]) {
				return p.annotationExpected("value")
			}
			p.i++
			for p.i < len(p.s) && (alpha(p.s[p.i]) || digit(p.s[p.i])) {
				p.i++
			}
			if !p.take('-') {
				break
			}
		}
		if !p.take(']') {
			return p.annotationExpected("close")
		}
		if critical {
			return errors.New("found unsupported RFC 9557 annotation with the critical flag (`!`) set")
		}
	}
	return nil
}

func escapeByte(b byte) string {
	switch b {
	case '\t':
		return `\t`
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case '\\':
		return `\\`
	case '\'':
		return `\'`
	case '"':
		return `\"`
	}
	if b < 32 || b >= 127 {
		return fmt.Sprintf(`\x%02X`, b)
	}
	return string(b)
}
func escapeBytes(s string) string {
	var out strings.Builder
	for len(s) > 0 {
		r, n := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && n == 1 {
			fmt.Fprintf(&out, `\x%02x`, s[0])
			s = s[1:]
			continue
		}
		s = s[n:]
		switch {
		case r == 0:
			out.WriteString(`\0`)
		case r < 128:
			out.WriteString(escapeByteLower(byte(r)))
		case !rustDebugPrintable(r):
			fmt.Fprintf(&out, `\u{%x}`, r)
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
func escapeByteLower(b byte) string {
	s := escapeByte(b)
	if strings.HasPrefix(s, `\x`) {
		return strings.ToLower(s)
	}
	return s
}

func parsedZonedResult(z Zoned, err error, tz TimeZone) (Zoned, error) {
	if err == nil {
		return z, nil
	}
	name := tz.Name()
	if name == "" {
		_, off := time.Unix(0, 0).In(tz.location()).Zone()
		name = offsetString(off, true, false, false)
	}
	return Zoned{}, fmt.Errorf("error converting datetime to instant in time zone %s: %s", name, err)
}
