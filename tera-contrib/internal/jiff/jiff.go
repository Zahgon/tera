// Package jiff implements the date, time zone, Temporal parsing and strtime
// behavior used by tera-contrib from Jiff 0.2.35.
package jiff

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const minSecond int64 = -377705023201
const maxSecond int64 = 253402207200
const secondRange = "parameter 'Unix timestamp seconds' is not in the required range of -377705023201..=253402207200"

// TimeZone represents a named zone or a fixed UTC offset. Its zero value is UTC.
type TimeZone struct {
	loc     *time.Location
	name    string
	unknown bool
	data    *zoneData
}

// UTC is the UTC time zone.
var UTC = TimeZone{loc: time.UTC, name: "UTC"}

func (tz TimeZone) location() *time.Location {
	if tz.loc == nil {
		return time.UTC
	}
	return tz.loc
}

// Name returns the zone identifier (including Etc/Unknown), or the empty
// string for a nonzero fixed offset.
func (tz TimeZone) Name() string {
	if tz.loc == nil {
		return "UTC"
	}
	return tz.name
}

// FixedTimeZone constructs a time zone with the given offset in seconds.
// The offset must be within Jiff's supported range, -93599..=93599.
func FixedTimeZone(offsetSeconds int32) TimeZone {
	if offsetSeconds == 0 {
		return UTC
	}
	return TimeZone{loc: time.FixedZone(offsetString(int(offsetSeconds), true, false, false), int(offsetSeconds))}
}

type cachedZone struct {
	zone     TimeZone
	expires  time.Time
	modified time.Time
}

var zoneCache = struct {
	sync.Mutex
	initialized bool
	directory   string
	names       map[string]string
	namesExpire time.Time
	zones       map[string]cachedZone
}{zones: map[string]cachedZone{}}

const zoneCacheTTL = 5 * time.Minute

// initZoneDatabase selects the process-wide database once, as Jiff does.
// The caller must hold zoneCache's lock.
func initZoneDatabase() {
	if zoneCache.initialized {
		return
	}
	zoneCache.initialized = true
	directories := []string{}
	if dir, ok := os.LookupEnv("TZDIR"); ok {
		directories = append(directories, dir)
	}
	directories = append(directories, "/usr/share/zoneinfo", "/usr/share/lib/zoneinfo", "/etc/zoneinfo")
	for _, candidate := range directories {
		// As in Jiff, a directory without a readable time zone is passed over for the next one; when none has
		// one, every lookup reports the zone as missing from the database.
		names, err := readZoneNames(candidate)
		if err != nil {
			continue
		}
		zoneCache.directory, zoneCache.names = candidate, names
		break
	}
	zoneCache.namesExpire = time.Now().Add(zoneCacheTTL)
}

func ensureZoneDatabase() {
	zoneCache.Lock()
	initZoneDatabase()
	zoneCache.Unlock()
}

// GetTimeZone looks up an IANA identifier, ignoring ASCII case, in TZDIR or
// the system zoneinfo database. UTC and Etc/Unknown are always available.
func GetTimeZone(name string) (TimeZone, error) {
	zoneCache.Lock()
	defer zoneCache.Unlock()
	initZoneDatabase()
	lower := asciiLower(name)
	if lower == "utc" {
		return UTC, nil
	}
	if lower == "etc/unknown" {
		return TimeZone{loc: time.UTC, name: "Etc/Unknown", unknown: true}, nil
	}
	now := time.Now()
	cached, hasCached := zoneCache.zones[lower]
	if hasCached && now.Before(cached.expires) {
		return cached.zone, nil
	}
	actual, ok := zoneCache.names[lower]
	if !ok && !now.Before(zoneCache.namesExpire) {
		// A failed rescan keeps the names read before, as Jiff's refresh does.
		if names, err := readZoneNames(zoneCache.directory); err == nil {
			zoneCache.names = names
		}
		zoneCache.namesExpire = now.Add(zoneCacheTTL)
		actual, ok = zoneCache.names[lower]
	}
	if !ok {
		return TimeZone{}, fmt.Errorf("failed to find time zone `%s` in time zone database", name)
	}
	path := filepath.Join(zoneCache.directory, actual)
	var modified time.Time
	if info, err := os.Stat(path); err == nil {
		modified = info.ModTime()
	}
	if hasCached && !modified.IsZero() && modified.Equal(cached.modified) {
		cached.expires = now.Add(zoneCacheTTL)
		zoneCache.zones[lower] = cached
		return cached.zone, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return TimeZone{}, err
	}
	metadata, err := readZoneData(data)
	if err != nil {
		return TimeZone{}, err
	}
	loc, err := time.LoadLocationFromTZData(actual, data)
	if err != nil {
		// The retained TZif representation also handles versions and footer
		// extensions that the host Go version may not understand.
		initial := metadata.types[0]
		loc = time.FixedZone(initial.abbreviation, initial.offset)
	}
	z := TimeZone{loc: loc, name: actual, data: metadata}
	zoneCache.zones[lower] = cachedZone{zone: z, expires: now.Add(zoneCacheTTL), modified: modified}
	return z, nil
}

// readZoneNames maps the lowercase name of every TZif file under dir to its name. Like Jiff's zoneinfo walk it
// collects as many names as it can: an entry that cannot be listed, opened or read is skipped, and the first such
// failure is returned only when no name could be read at all (or an error saying that dir holds no TZif file).
func readZoneNames(dir string) (map[string]string, error) {
	names := map[string]string{}
	walkDir := dir
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		walkDir = resolved
	}
	var firstErr error
	skip := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}
	err := filepath.WalkDir(walkDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			skip(err)
			return nil
		}
		rel, err := filepath.Rel(walkDir, path)
		if err != nil {
			skip(err)
			return nil
		}
		if d.IsDir() {
			if rel == "posix" || rel == "right" {
				return filepath.SkipDir
			}
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			skip(err)
			return nil
		}
		var magic [4]byte
		n, err := file.Read(magic[:])
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		switch {
		case err == nil && n == 4 && string(magic[:]) == "TZif":
			names[asciiLower(filepath.ToSlash(rel))] = filepath.ToSlash(rel)
		case err != nil && !errors.Is(err, io.EOF):
			skip(err)
		}
		return nil
	})
	if err != nil {
		skip(err)
	}
	if len(names) == 0 {
		if firstErr == nil {
			firstErr = errors.New("did not find any TZif files in zoneinfo time zone database")
		}
		return nil, firstErr
	}
	return names, nil
}

// Timestamp is an instant with nanosecond resolution.
type Timestamp struct{ t time.Time }

// NewTimestamp constructs an instant from Unix seconds plus signed nanoseconds.
func NewTimestamp(second int64, nanosecond int32) (Timestamp, error) {
	if second < minSecond || second > maxSecond {
		return Timestamp{}, errors.New(secondRange)
	}
	if nanosecond < -999999999 || nanosecond > 999999999 {
		return Timestamp{}, errors.New("parameter 'subsecond nanosecond' is not in the required range of -999999999..=999999999")
	}
	if second == minSecond && nanosecond < 0 {
		return Timestamp{}, errors.New(secondRange)
	}
	return Timestamp{time.Unix(second, int64(nanosecond)).UTC()}, nil
}

// String prints the timestamp using Jiff's Temporal representation.
func (t Timestamp) String() string { return civilString(t.t.UTC(), true) + "Z" }

// ToZoned attaches a time zone to an instant.
func (t Timestamp) ToZoned(tz TimeZone) Zoned { return Zoned{t: t.t, tz: tz} }

// Zoned is an instant with an attached time zone.
type Zoned struct {
	t        time.Time
	tz       TimeZone
	override *int
}

// WithTimeZone attaches a different time zone to the same instant.
func (z Zoned) WithTimeZone(tz TimeZone) Zoned { z.tz = tz; z.override = nil; return z }

// Compare compares instants, independently of their attached time zones.
func (z Zoned) Compare(other Zoned) int { return z.t.Compare(other.t) }

// GoTime returns the corresponding standard library time in the attached zone.
func (z Zoned) GoTime() time.Time {
	info := z.tz.info(z.t)
	if z.override != nil {
		info.offset = *z.override
	}
	t := z.t.In(z.tz.location())
	name, off := t.Zone()
	if off == info.offset && name == info.abbreviation {
		return t
	}
	return z.t.In(time.FixedZone(info.abbreviation, info.offset))
}

// String prints Jiff's RFC 9557 representation of a zoned instant.
func (z Zoned) String() string {
	t := z.GoTime()
	_, off := t.Zone()
	name := z.tz.Name()
	if name == "" {
		name = roundedOffset(off)
	}
	o := roundedOffset(off)
	if z.tz.unknown {
		o = "Z"
	}
	return civilString(t, true) + o + "[" + name + "]"
}

// DateTime is a civil date and time without a time zone.
type DateTime struct{ t time.Time }

// Date is a civil calendar date without a time zone.
type Date struct{ t time.Time }

// String prints a civil datetime in Temporal format.
func (d DateTime) String() string { return civilString(d.t, true) }

// String prints a civil date in Temporal format.
func (d Date) String() string { return civilString(d.t, false) }

// ToZoned resolves a civil date at midnight using compatible disambiguation.
func (d Date) ToZoned(tz TimeZone) (Zoned, error) { return DateTime{d.t}.ToZoned(tz) }

// ToZoned resolves a civil time, choosing the earlier instant in a fold and
// moving forward by the transition length in a gap.
func (d DateTime) ToZoned(tz TimeZone) (Zoned, error) {
	amb := tz.ambiguous(d.t)
	z, err := civilToZoned(d.t, amb.before, tz)
	if err != nil {
		return Zoned{}, err
	}
	if amb.kind == 0 {
		offset := amb.before
		z.override = &offset
	}
	return z, nil
}

func civilToZoned(dt time.Time, offset int, tz TimeZone) (Zoned, error) {
	t := time.Unix(dt.Unix()-int64(offset), int64(dt.Nanosecond()))
	if t.Unix() < minSecond || t.Unix() > maxSecond {
		return Zoned{}, fmt.Errorf("converting datetime with time zone offset `%s` to timestamp overflowed: %s", offsetString(offset, true, false, false), secondRange)
	}
	return Zoned{t: t, tz: tz}, nil
}

func civilString(t time.Time, withTime bool) string {
	y := t.Year()
	ys := fmt.Sprintf("%04d", y)
	if y < 0 {
		ys = fmt.Sprintf("-%06d", -y)
	}
	s := fmt.Sprintf("%s-%02d-%02d", ys, t.Month(), t.Day())
	if !withTime {
		return s
	}
	s += fmt.Sprintf("T%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
	if t.Nanosecond() != 0 {
		s += "." + strings.TrimRight(fmt.Sprintf("%09d", t.Nanosecond()), "0")
	}
	return s
}

func offsetString(off int, colon, minute, second bool) string {
	sign := "+"
	if off < 0 {
		sign = "-"
		off = -off
	}
	h, m, s := off/3600, (off/60)%60, off%60
	out := fmt.Sprintf("%s%02d", sign, h)
	sep := ""
	if colon {
		sep = ":"
	}
	if minute || m != 0 || s != 0 {
		out += fmt.Sprintf("%s%02d", sep, m)
		if second || s != 0 {
			out += fmt.Sprintf("%s%02d", sep, s)
		}
	}
	return out
}
func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return r
	}, s)
}
func asciiUpper(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r - 32
		}
		return r
	}, s)
}

func roundedOffset(off int) string {
	sign := "+"
	if off < 0 {
		sign = "-"
		off = -off
	}
	minutes := (off + 30) / 60
	if minutes > 25*60+59 {
		minutes = 25*60 + 59
	}
	return fmt.Sprintf("%s%02d:%02d", sign, minutes/60, minutes%60)
}
