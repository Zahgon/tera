// Package base64 reproduces the general-purpose engines of Rust's base64 crate.
package base64

import (
	stdbase64 "encoding/base64"
	"fmt"
	"unicode/utf8"
)

// Encode encodes data using the selected alphabet and padding policy.
func Encode(data []byte, urlSafe, padded bool) string {
	e := stdbase64.StdEncoding
	if urlSafe {
		e = stdbase64.URLEncoding
	}
	if !padded {
		e = e.WithPadding(stdbase64.NoPadding)
	}
	return e.EncodeToString(data)
}

// Decode accepts absent or partial terminal padding, and rejects trailing bits.
func Decode(s string, urlSafe bool) ([]byte, error) {
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	if urlSafe {
		alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	}
	var table [256]int
	for i := range table {
		table[i] = -1
	}
	for i := range alphabet {
		table[alphabet[i]] = i
	}
	invalid := func(i int) error { return fmt.Errorf("Invalid symbol %d, offset %d.", s[i], i) }
	if len(s)%4 == 1 && s[len(s)-1] != '=' && table[s[len(s)-1]] < 0 {
		return nil, invalid(len(s) - 1)
	}
	prefix := len(s) / 4 * 4
	if len(s)%4 == 0 && prefix > 0 {
		prefix -= 4
	}
	out := make([]byte, 0, len(s)/4*3+3)
	for i := 0; i < prefix; i += 4 {
		var v uint32
		for j := 0; j < 4; j++ {
			x := table[s[i+j]]
			if x < 0 {
				return nil, invalid(i + j)
			}
			v = v<<6 | uint32(x)
		}
		out = append(out, byte(v>>16), byte(v>>8), byte(v))
	}
	count, pad := 0, -1
	var v uint32
	var last byte
	for i := prefix; i < len(s); i++ {
		b := s[i]
		if b == '=' {
			if i-prefix < 2 {
				return nil, invalid(i)
			}
			if pad < 0 {
				pad = i
			}
			continue
		}
		if pad >= 0 {
			return nil, invalid(pad)
		}
		x := table[b]
		if x < 0 {
			return nil, invalid(i)
		}
		v = v<<6 | uint32(x)
		count++
		last = b
	}
	if len(s) > 0 && count < 2 {
		return nil, fmt.Errorf("Invalid input length: %d", prefix+count)
	}
	if count == 2 && v&15 != 0 || count == 3 && v&3 != 0 {
		return nil, fmt.Errorf("Invalid last symbol 0x%02x ('%c') at offset %d, decoded as 0b%08b.", last, last, prefix+count-1, table[last])
	}
	if count >= 2 {
		v <<= uint(24 - count*6)
		out = append(out, byte(v>>16))
		if count >= 3 {
			out = append(out, byte(v>>8))
		}
		if count == 4 {
			out = append(out, byte(v))
		}
	}
	return out, nil
}

// UTF8Error returns Rust's String::from_utf8 error, or nil for valid UTF-8.
func UTF8Error(b []byte) error {
	for i := 0; i < len(b); {
		x := b[i]
		if x < 128 {
			i++
			continue
		}
		n := 0
		switch {
		case x >= 0xc2 && x <= 0xdf:
			n = 2
		case x >= 0xe0 && x <= 0xef:
			n = 3
		case x >= 0xf0 && x <= 0xf4:
			n = 4
		}
		bad := func(n int) error { return fmt.Errorf("invalid utf-8 sequence of %d bytes from index %d", n, i) }
		if n == 0 {
			return bad(1)
		}
		for j := 1; j < n; j++ {
			if i+j >= len(b) {
				return fmt.Errorf("incomplete utf-8 byte sequence from index %d", i)
			}
			c := b[i+j]
			if c < 0x80 || c > 0xbf || j == 1 && (x == 0xe0 && c < 0xa0 || x == 0xed && c >= 0xa0 || x == 0xf0 && c < 0x90 || x == 0xf4 && c >= 0x90) {
				return bad(j)
			}
		}
		_, size := utf8.DecodeRune(b[i:])
		i += size
	}
	return nil
}
