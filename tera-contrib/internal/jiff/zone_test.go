package jiff

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCustomZoneInfo pins Rust-derived behavior for compact TZif files whose
// initial types and footers differ from the standard library loader.
func TestCustomZoneInfo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TZDIR", dir)
	files := map[string][]byte{
		"InitialDST":   testZoneFile([]zoneInfo{{3600, "DST", true}, {0, "STD", false}}, []zoneTransition{{second: 0, typ: 0}, {second: 100000, typ: 1}}, nil),
		"FooterDST":    testZoneFile([]zoneInfo{{0, "STD", false}}, nil, testFooter("STD0DST,M3.2.0,M11.1.0")),
		"YearWrap":     testZoneFile([]zoneInfo{{0, "STD", false}}, nil, testFooter("STD0DST,J365/26,J1/26")),
		"Inconsistent": testZoneFile([]zoneInfo{{3600, "BAD", false}}, []zoneTransition{{second: 0, typ: 0}}, testFooter("STD0")),
		"BadFooter":    testZoneFile([]zoneInfo{{0, "STD", false}}, nil, testFooter("BAD")),
		"BigOffset":    testZoneFile([]zoneInfo{{100000, "BIG", false}}, nil, nil),
		"BadUTF8":      testZoneFile([]zoneInfo{{0, "\xff", false}}, nil, nil),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Each probe process chooses its database before parsing its cases.
	zoneCache.Lock()
	oldInitialized, oldDirectory, oldNames, oldExpire, oldZones := zoneCache.initialized, zoneCache.directory, zoneCache.names, zoneCache.namesExpire, zoneCache.zones
	zoneCache.initialized = false
	zoneCache.zones = map[string]cachedZone{}
	zoneCache.Unlock()
	t.Cleanup(func() {
		zoneCache.Lock()
		zoneCache.initialized, zoneCache.directory, zoneCache.names, zoneCache.namesExpire, zoneCache.zones = oldInitialized, oldDirectory, oldNames, oldExpire, oldZones
		zoneCache.Unlock()
	})
	for _, name := range []string{"Inconsistent", "BadFooter", "BigOffset", "BadUTF8"} {
		if _, err := GetTimeZone(name); err == nil {
			t.Fatalf("expected %s to be rejected", name)
		}
	}
	zone, err := GetTimeZone("initialdst")
	if err != nil {
		t.Fatal(err)
	}
	ts, err := NewTimestamp(-1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ts.ToZoned(zone).String(), "1970-01-01T00:59:59+01:00[InitialDST]"; got != want {
		t.Fatalf("ToZoned(InitialDST) = %q, want %q", got, want)
	}
	for _, tt := range []struct{ zone, input, zoned, civil string }{
		{"FooterDST", "1800-03-10T00:30[FooterDST]", "1800-03-10T00:30:00+01:00[FooterDST]", "1800-03-10T00:30:00+01:00[FooterDST]"},
		{"YearWrap", "1800-01-01T00:30[YearWrap]", "1799-12-31T23:30:00+00:00[YearWrap]", "1800-01-01T00:30:00+01:00[YearWrap]"},
	} {
		z, err := ParseZoned(tt.input)
		if err != nil {
			t.Fatal(err)
		}
		if z.String() != tt.zoned {
			t.Fatalf("got %s; want %s", z, tt.zoned)
		}
		d, err := ParseDateTime(tt.input)
		if err != nil {
			t.Fatal(err)
		}
		tz, err := GetTimeZone(tt.zone)
		if err != nil {
			t.Fatal(err)
		}
		z, err = d.ToZoned(tz)
		if err != nil {
			t.Fatal(err)
		}
		if z.String() != tt.civil {
			t.Fatalf("got %s; want %s", z, tt.civil)
		}
		if z.WithTimeZone(tz).String() != tt.zoned {
			t.Fatal("WithTimeZone did not recompute local fields")
		}
	}
	// A new alias is discovered after the name cache expires.
	if err := os.WriteFile(filepath.Join(dir, "Added"), files["InitialDST"], 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := GetTimeZone("Added"); err == nil {
		t.Fatal("fresh name cache unexpectedly refreshed")
	}
	zoneCache.Lock()
	zoneCache.namesExpire = time.Time{}
	zoneCache.Unlock()
	if _, err := GetTimeZone("Added"); err != nil {
		t.Fatal(err)
	}
	// Expired entries reload TZif bytes when their modification time changes.
	if err := os.WriteFile(filepath.Join(dir, "InitialDST"), testZoneFile([]zoneInfo{{7200, "NEW", false}}, nil, nil), 0600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "InitialDST"), future, future); err != nil {
		t.Fatal(err)
	}
	zoneCache.Lock()
	cached := zoneCache.zones["initialdst"]
	cached.expires = time.Time{}
	zoneCache.zones["initialdst"] = cached
	zoneCache.Unlock()
	updated, err := GetTimeZone("InitialDST")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ts.ToZoned(updated).String(), "1970-01-01T01:59:59+02:00[InitialDST]"; got != want {
		t.Fatalf("ToZoned(reloaded InitialDST) = %q, want %q", got, want)
	}

}

func testFooter(s string) *string { return &s }
func testZoneFile(types []zoneInfo, transitions []zoneTransition, footer *string) []byte {
	var names []byte
	var records []byte
	for _, typ := range types {
		records = binary.BigEndian.AppendUint32(records, uint32(typ.offset))
		dst := byte(0)
		if typ.dst {
			dst = 1
		}
		records = append(records, dst, byte(len(names)))
		names = append(names, typ.abbreviation...)
		names = append(names, 0)
	}
	header := func() []byte {
		b := make([]byte, 20)
		copy(b, "TZif")
		if footer != nil {
			b[4] = '2'
		}
		for _, n := range []int{0, 0, 0, len(transitions), len(types), len(names)} {
			b = binary.BigEndian.AppendUint32(b, uint32(n))
		}
		return b
	}
	block := func(size int) []byte {
		b := []byte{}
		for _, tr := range transitions {
			if size == 4 {
				b = binary.BigEndian.AppendUint32(b, uint32(tr.second))
			} else {
				b = binary.BigEndian.AppendUint64(b, uint64(tr.second))
			}
		}
		for _, tr := range transitions {
			b = append(b, byte(tr.typ))
		}
		b = append(b, records...)
		b = append(b, names...)
		return b
	}
	b := header()
	b = append(b, block(4)...)
	if footer != nil {
		b = append(b, header()...)
		b = append(b, block(8)...)
		b = append(b, '\n')
		b = append(b, (*footer)...)
		b = append(b, '\n')
	}
	return b
}
