package jiff

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"
	"time"
	"unicode/utf8"
)

// zoneData retains the TZif facts whose interpretation differs from Go's
// historical first-zone heuristic and POSIX footer evaluation.
type zoneData struct {
	types       []zoneInfo
	transitions []zoneTransition
	footer      *posixZone
}
type zoneInfo struct {
	offset       int
	abbreviation string
	dst          bool
}
type zoneTransition struct {
	second     int64
	typ        int
	start, end time.Time
	kind       byte
}
type ambiguity struct {
	before, after int
	kind          byte
}

var errZoneData = errors.New("invalid TZif time zone data")

func readZoneData(data []byte) (*zoneData, error) {
	block := func(at, size int) (counts [6]int, start, end int, ok bool) {
		if at < 0 || len(data)-at < 44 || string(data[at:at+4]) != "TZif" {
			return
		}
		for i := range counts {
			counts[i] = int(binary.BigEndian.Uint32(data[at+20+i*4 : at+24+i*4]))
		}
		g, s, l, n, t, c := counts[0], counts[1], counts[2], counts[3], counts[4], counts[5]
		if t < 1 || c < 1 || (g != 0 && g != t) || (s != 0 && s != t) {
			return
		}
		length := int64(n)*int64(size+1) + int64(t)*6 + int64(c) + int64(l)*int64(size+4) + int64(g+s)
		start = at + 44
		if length > int64(len(data)-start) {
			return
		}
		end = start + int(length)
		ok = true
		return
	}
	counts, start, end, ok := block(0, 4)
	if !ok {
		return nil, errZoneData
	}
	size := 4
	v2 := data[4] != 0
	if v2 {
		counts, start, end, ok = block(end, 8)
		if !ok {
			return nil, errZoneData
		}
		size = 8
	}
	g, s, l, n, t, c := counts[0], counts[1], counts[2], counts[3], counts[4], counts[5]
	times := data[start : start+n*size]
	indices := data[start+n*size : start+n*(size+1)]
	records := data[start+n*(size+1) : start+n*(size+1)+t*6]
	namesAt := start + n*(size+1) + t*6
	names := data[namesAt : namesAt+c]
	if !utf8.Valid(names) {
		return nil, errZoneData
	}
	z := &zoneData{}
	for i := 0; i < t; i++ {
		rec := records[i*6 : i*6+6]
		off := int(int32(binary.BigEndian.Uint32(rec)))
		a := int(rec[5])
		if off < -93599 || off > 93599 || a >= len(names) || !utf8.RuneStart(names[a]) {
			return nil, errZoneData
		}
		length := bytes.IndexByte(names[a:], 0)
		if length < 0 || a+length > 255 {
			return nil, errZoneData
		}
		z.types = append(z.types, zoneInfo{off, string(names[a : a+length]), rec[4] == 1})
	}
	z.transitions = append(z.transitions, zoneTransition{second: minSecond, typ: 0})
	for i := 0; i < n; i++ {
		var sec int64
		if size == 4 {
			sec = int64(int32(binary.BigEndian.Uint32(times[i*4 : i*4+4])))
		} else {
			sec = int64(binary.BigEndian.Uint64(times[i*8 : i*8+8]))
		}
		if sec < minSecond {
			sec = minSecond
		} else if sec > maxSecond {
			sec = maxSecond
		}
		typ := int(indices[i])
		if typ >= t {
			return nil, errZoneData
		}
		z.transitions = append(z.transitions, zoneTransition{second: sec, typ: typ})
	}
	indicatorsAt := namesAt + c + l*(size+4)
	standard := data[indicatorsAt : indicatorsAt+s]
	universal := data[indicatorsAt+s : indicatorsAt+s+g]
	if s == 0 {
		for _, v := range universal {
			if v != 0 {
				return nil, errZoneData
			}
		}
	} else {
		for i, v := range standard {
			if v > 1 {
				return nil, errZoneData
			}
			if g > 0 && (universal[i] > 1 || universal[i] == 1 && v == 0) {
				return nil, errZoneData
			}
		}
	}
	if v2 {
		tail := data[end:]
		if len(tail) < 2 || tail[0] != '\n' {
			return nil, errZoneData
		}
		limit := len(tail) - 1
		if limit > 1024 {
			limit = 1024
		}
		nl := bytes.IndexByte(tail[1:1+limit], '\n')
		if nl < 0 {
			return nil, errZoneData
		}
		if nl > 0 {
			var err error
			z.footer, err = parsePOSIX(string(tail[1 : 1+nl]))
			if err != nil {
				return nil, err
			}
		}
	}
	if z.footer != nil {
		// Jiff's default tz-fat feature expands up to 301 transitions, stopping
		// before 2038. Keeping these transitions also preserves its boundary rules.
		for i := 0; i <= 300; i++ {
			last := z.transitions[len(z.transitions)-1].second
			next, info, ok := z.footer.next(last)
			if !ok || next.Year() >= 2038 {
				break
			}
			typ := -1
			for j, existing := range z.types {
				if existing == info {
					typ = j
					break
				}
			}
			if typ < 0 {
				if len(z.types) >= 256 {
					break
				}
				typ = len(z.types)
				z.types = append(z.types, info)
			}
			second := next.Unix()
			if second < 0 && next.Nanosecond() != 0 {
				second++
			}
			z.transitions = append(z.transitions, zoneTransition{second: second, typ: typ})
		}
		if len(z.transitions) > 1 {
			last := z.transitions[len(z.transitions)-1]
			info := z.footer.info(time.Unix(last.second, 0).UTC())
			if info != z.types[last.typ] {
				return nil, errZoneData
			}
		}
	}
	civilMin := time.Date(-9999, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range z.transitions {
		tr := &z.transitions[i]
		if i == 0 {
			tr.start, tr.end = civilMin, civilMin
			continue
		}
		before, after := z.types[z.transitions[i-1].typ].offset, z.types[tr.typ].offset
		tr.start = time.Unix(tr.second+int64(before), 0).UTC()
		tr.end = time.Unix(tr.second+int64(after), 0).UTC()
		if before < after {
			tr.kind = 'g'
		} else if before > after {
			tr.kind = 'f'
			tr.start, tr.end = tr.end, tr.start
		}
	}
	return z, nil
}

func (z *zoneData) info(t time.Time) zoneInfo {
	sec := t.Unix()
	i := sort.Search(len(z.transitions), func(i int) bool { return z.transitions[i].second > sec }) - 1
	if i < 0 {
		i = 0
	}
	if i == len(z.transitions)-1 && z.footer != nil {
		return z.footer.info(t.UTC())
	}
	return z.types[z.transitions[i].typ]
}
func (z *zoneData) ambiguous(dt time.Time) ambiguity {
	i := sort.Search(len(z.transitions), func(i int) bool { return z.transitions[i].start.After(dt) }) - 1
	if i < 0 {
		i = 0
	}
	tr := z.transitions[i]
	after := z.types[tr.typ].offset
	if i > 0 && tr.kind != 0 && dt.Before(tr.end) {
		return ambiguity{z.types[z.transitions[i-1].typ].offset, after, tr.kind}
	}
	if i == len(z.transitions)-1 && z.footer != nil {
		return z.footer.ambiguous(dt)
	}
	return ambiguity{before: after, after: after}
}
func (tz TimeZone) info(t time.Time) zoneInfo {
	if tz.data != nil {
		return tz.data.info(t)
	}
	name, off := t.In(tz.location()).Zone()
	return zoneInfo{off, name, t.In(tz.location()).IsDST()}
}
func (tz TimeZone) ambiguous(dt time.Time) ambiguity {
	if tz.data != nil {
		return tz.data.ambiguous(dt)
	}
	info := tz.info(dt)
	return ambiguity{before: info.offset, after: info.offset}
}

type posixZone struct {
	std, dst   zoneInfo
	hasDST     bool
	start, end posixRule
}
type posixRule struct {
	kind                              byte
	day, month, week, weekday, second int
}
type posixParser struct {
	s string
	i int
}

func (p *posixParser) take(b byte) bool {
	if p.i < len(p.s) && p.s[p.i] == b {
		p.i++
		return true
	}
	return false
}
func (p *posixParser) number(maxDigits, min, max int, exact bool) (int, bool) {
	start := p.i
	v := 0
	for p.i < len(p.s) && p.i-start < maxDigits && digit(p.s[p.i]) {
		v = v*10 + int(p.s[p.i]-'0')
		p.i++
	}
	return v, p.i > start && (!exact || p.i-start == maxDigits) && v >= min && v <= max
}
func (p *posixParser) abbreviation() (string, bool) {
	quoted := p.take('<')
	start := p.i
	for p.i < len(p.s) && (alpha(p.s[p.i]) || quoted && (digit(p.s[p.i]) || p.s[p.i] == '+' || p.s[p.i] == '-')) {
		p.i++
	}
	s := p.s[start:p.i]
	if len(s) < 3 || len(s) > 30 {
		return "", false
	}
	if quoted && !p.take('>') {
		return "", false
	}
	return s, true
}
func (p *posixParser) clock(transition bool) (int, bool) {
	sign := 1
	if p.take('-') {
		sign = -1
	} else {
		p.take('+')
	}
	digits, max := 2, 24
	if transition {
		digits, max = 3, 167
	}
	h, ok := p.number(digits, 0, max, false)
	if !ok {
		return 0, false
	}
	mi, se := 0, 0
	if p.take(':') {
		mi, ok = p.number(2, 0, 59, true)
		if !ok {
			return 0, false
		}
		if p.take(':') {
			se, ok = p.number(2, 0, 59, true)
			if !ok {
				return 0, false
			}
		}
	}
	v := sign * (h*3600 + mi*60 + se)
	if !transition {
		v = -v
	}
	return v, true
}
func (p *posixParser) rule() (posixRule, bool) {
	r := posixRule{second: 7200}
	var ok bool
	if p.take('M') {
		r.kind = 'M'
		r.month, ok = p.number(2, 1, 12, false)
		if !ok || !p.take('.') {
			return r, false
		}
		r.week, ok = p.number(1, 1, 5, true)
		if !ok || !p.take('.') {
			return r, false
		}
		r.weekday, ok = p.number(1, 0, 6, true)
	} else if p.take('J') {
		r.kind = 'J'
		r.day, ok = p.number(3, 1, 365, false)
	} else {
		r.kind = '0'
		r.day, ok = p.number(3, 0, 365, false)
	}
	if !ok {
		return r, false
	}
	if p.take('/') {
		r.second, ok = p.clock(true)
	}
	return r, ok
}
func parsePOSIX(s string) (*posixZone, error) {
	p := posixParser{s: s}
	z := &posixZone{}
	var ok bool
	z.std.abbreviation, ok = p.abbreviation()
	if !ok {
		return nil, errZoneData
	}
	z.std.offset, ok = p.clock(false)
	if !ok {
		return nil, errZoneData
	}
	if p.i == len(s) {
		return z, nil
	}
	z.dst.abbreviation, ok = p.abbreviation()
	if !ok {
		return nil, errZoneData
	}
	z.hasDST = true
	z.dst.dst = true
	z.dst.offset = z.std.offset + 3600
	if p.i < len(s) && s[p.i] != ',' {
		z.dst.offset, ok = p.clock(false)
		if !ok {
			return nil, errZoneData
		}
	}
	if !p.take(',') {
		return nil, errZoneData
	}
	z.start, ok = p.rule()
	if !ok || !p.take(',') {
		return nil, errZoneData
	}
	z.end, ok = p.rule()
	if !ok || p.i != len(s) {
		return nil, errZoneData
	}
	return z, nil
}
func (r posixRule) datetime(year, offset int) time.Time {
	min := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	max := time.Date(year, 12, 31, 23, 59, 59, 999999999, time.UTC)
	var dt time.Time
	switch r.kind {
	case 'M':
		first := time.Date(year, time.Month(r.month), 1, 0, 0, 0, 0, time.UTC)
		day := 1 + (r.weekday-int(first.Weekday())+7)%7 + (r.week-1)*7
		if r.week == 5 && day > time.Date(year, time.Month(r.month)+1, 0, 0, 0, 0, 0, time.UTC).Day() {
			day -= 7
		}
		dt = time.Date(year, time.Month(r.month), day, 0, 0, 0, 0, time.UTC)
	case 'J':
		day := r.day - 1
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) && r.day >= 60 {
			day++
		}
		dt = min.AddDate(0, 0, day)
	default:
		dt = min.AddDate(0, 0, r.day)
		if dt.Year() != year {
			return max
		}
	}
	dt = dt.Add(time.Duration(r.second-offset) * time.Second)
	if dt.Before(min) {
		return min
	}
	if dt.After(max) {
		return max
	}
	return dt
}
func (p *posixZone) bounds(year int, utc bool) (time.Time, time.Time) {
	off1, off2 := 0, 0
	if utc {
		off1, off2 = p.std.offset, p.dst.offset
	}
	start, end := p.start.datetime(year, off1), p.end.datetime(year, off2)
	if utc && start.Month() == 1 && start.Day() == 1 && start.Hour() == 0 && start.Minute() == 0 && start.Second() == 0 && start.Nanosecond() == 0 && end.Add(time.Duration(p.std.offset)*time.Second).Year() != year {
		end = time.Date(year, 12, 31, 23, 59, 59, 999999999, time.UTC)
	}
	return start, end
}
func inDST(dt, start, end time.Time) bool {
	if !start.After(end) {
		return !dt.Before(start) && dt.Before(end)
	}
	return dt.Before(end) || !dt.Before(start)
}
func (p *posixZone) info(t time.Time) zoneInfo {
	if p.hasDST {
		start, end := p.bounds(t.Year(), true)
		if inDST(t, start, end) {
			return p.dst
		}
	}
	return p.std
}
func (p *posixZone) next(second int64) (time.Time, zoneInfo, bool) {
	if !p.hasDST {
		return time.Time{}, zoneInfo{}, false
	}
	dt := time.Unix(second, 0).UTC()
	start, end := p.bounds(dt.Year(), true)
	if start.After(end) {
		start, end = end, start
	}
	next := start
	if !dt.Before(start) {
		next = end
		if !dt.Before(end) {
			if dt.Year() == 9999 {
				return time.Time{}, zoneInfo{}, false
			}
			a, b := p.bounds(dt.Year()+1, true)
			next = a
			if b.Before(a) {
				next = b
			}
		}
	}
	if next.Unix() < minSecond || next.Unix() > maxSecond {
		return time.Time{}, zoneInfo{}, false
	}
	return next, p.info(next), true
}
func (p *posixZone) ambiguous(dt time.Time) ambiguity {
	std, dst := p.std.offset, p.dst.offset
	a := ambiguity{before: std, after: std}
	if !p.hasDST || std == dst {
		return a
	}
	start, end := p.bounds(dt.Year(), false)
	diff := dst - std
	inside := inDST(dt, start, end)
	if diff < 0 {
		if inside {
			return ambiguity{before: dst, after: dst}
		}
		foldStart := start.Add(time.Duration(diff) * time.Second)
		gapEnd := end.Add(time.Duration(-diff) * time.Second)
		if !dt.Before(foldStart) && dt.Before(start) {
			return ambiguity{std, dst, 'f'}
		}
		if !dt.Before(end) && dt.Before(gapEnd) {
			return ambiguity{dst, std, 'g'}
		}
		return a
	}
	if !inside {
		return a
	}
	gapEnd := start.Add(time.Duration(diff) * time.Second)
	foldStart := end.Add(time.Duration(-diff) * time.Second)
	if !dt.Before(start) && dt.Before(gapEnd) {
		return ambiguity{std, dst, 'g'}
	}
	if !dt.Before(foldStart) && dt.Before(end) {
		return ambiguity{dst, std, 'f'}
	}
	return ambiguity{before: dst, after: dst}
}
