// Package percent implements the ASCII sets and byte encoding of percent-encoding.
package percent

import "strings"

// AsciiSet is an immutable, copyable set of ASCII bytes to percent-encode.
type AsciiSet [2]uint64

// Add returns a set that also encodes b. Non-ASCII bytes are always encoded.
func (s AsciiSet) Add(b byte) AsciiSet {
	if b < 128 {
		s[b/64] |= uint64(1) << (b % 64)
	}
	return s
}

// Remove returns a set that does not encode ASCII byte b.
func (s AsciiSet) Remove(b byte) AsciiSet {
	if b < 128 {
		s[b/64] &^= uint64(1) << (b % 64)
	}
	return s
}

// CONTROLS encodes ASCII controls and DEL.
var CONTROLS = func() AsciiSet {
	var s AsciiSet
	for i := 0; i < 32; i++ {
		s = s.Add(byte(i))
	}
	return s.Add(127)
}()

// NON_ALPHANUMERIC encodes every ASCII byte except letters and decimal digits.
var NON_ALPHANUMERIC = func() AsciiSet {
	var s AsciiSet
	for i := 0; i < 128; i++ {
		if !(i >= 'a' && i <= 'z' || i >= 'A' && i <= 'Z' || i >= '0' && i <= '9') {
			s = s.Add(byte(i))
		}
	}
	return s
}()

// Encode percent-encodes s as UTF-8 bytes, with uppercase hex digits.
func Encode(s string, set AsciiSet) string {
	const hex = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 128 || set[b/64]&(uint64(1)<<(b%64)) != 0 {
			out.WriteByte('%')
			out.WriteByte(hex[b>>4])
			out.WriteByte(hex[b&15])
		} else {
			out.WriteByte(b)
		}
	}
	return out.String()
}
