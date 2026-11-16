package jiff

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Format formats a zoned instant using Jiff's locale-independent strtime
// conversion specifications, including padding, case, width and precision.
func Format(format string, z Zoned) (string, error) {
	s, err := formatTime(format, z)
	if err != nil {
		return "", fmt.Errorf("strftime formatting failed: %s", err)
	}
	return s, nil
}

type extension struct {
	flag   byte
	width  int
	colons int
}

func parseWidth(s string, i *int) (int, error) {
	start := *i
	for *i < len(s) && digit(s[*i]) {
		*i = *i + 1
	}
	if *i == start {
		return -1, nil
	}
	width, err := strconv.ParseInt(s[start:*i], 10, 64)
	if err != nil {
		return 0, errors.New("failed to parse conversion specifier width: number too big to parse into 64-bit integer")
	}
	if width > 255 {
		return 0, errors.New("parsed width is too big, max is 255")
	}
	if *i == len(s) {
		return 0, errors.New("expected to find specifier directive after parsed width, but found end of format string")
	}
	return int(width), nil
}
func formatTime(s string, z Zoned) (string, error) {
	var out strings.Builder
	t := z.GoTime()
	y, month, day := t.Date()
	h, mi, se := t.Clock()
	ns := t.Nanosecond()
	wd := int(t.Weekday())
	iy, iw := t.ISOWeek()
	_, off := t.Zone()
	h12 := h % 12
	if h12 == 0 {
		h12 = 12
	}
	for i := 0; i < len(s); {
		if s[i] != '%' {
			r, n := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && n == 1 {
				return "", errors.New("invalid format string, it must be valid UTF-8")
			}
			out.WriteString(s[i : i+n])
			i += n
			continue
		}
		i++
		if i == len(s) {
			return "", errors.New("invalid format string, expected byte after `%`, but found end of format string")
		}
		ext := extension{width: -1}
		if strings.ContainsRune("_0-^#", rune(s[i])) {
			ext.flag = s[i]
			i++
			if i == len(s) {
				return "", fmt.Errorf("expected to find specifier directive after flag `%s`, but found end of format string", escapeByte(ext.flag))
			}
		}
		var err error
		ext.width, err = parseWidth(s, &i)
		if err != nil {
			return "", err
		}
		for ext.colons < 3 && i < len(s) && s[i] == ':' {
			ext.colons++
			i++
		}
		if i == len(s) {
			return "", errors.New("expected to find specifier directive after colons, but found end of format string")
		}
		c := s[i]
		i++
		v := int64(0)
		pad := byte('0')
		width := 2
		numeric := true
		value := ""
		applyCase := true
		switch c {
		case '%':
			numeric = false
			value = "%"
		case 'A':
			numeric = false
			value = t.Weekday().String()
		case 'a':
			numeric = false
			value = t.Weekday().String()[:3]
		case 'B':
			numeric = false
			value = month.String()
		case 'b', 'h':
			numeric = false
			value = month.String()[:3]
		case 'C':
			v = int64(y / 100)
			pad = ' '
			width = 0
		case 'd':
			v = int64(day)
		case 'e':
			v = int64(day)
			pad = ' '
		case 'G':
			v = int64(iy)
			width = 4
		case 'g':
			v = int64(iy % 100)
		case 'H':
			v = int64(h)
		case 'I':
			v = int64(h12)
		case 'j':
			v = int64(t.YearDay())
			width = 3
		case 'k':
			v = int64(h)
			pad = ' '
		case 'l':
			v = int64(h12)
			pad = ' '
		case 'M':
			v = int64(mi)
		case 'm':
			v = int64(month)
		case 'n':
			numeric = false
			value = "\n"
		case 't':
			numeric = false
			value = "\t"
		case 'p', 'P':
			numeric = false
			value = "AM"
			if h >= 12 {
				value = "PM"
			}
			if c == 'P' || ext.flag == '#' {
				value = asciiLower(value)
			}
		case 'q':
			v = int64((int(month)-1)/3 + 1)
			width = 0
		case 'S':
			v = int64(se)
		case 's':
			v = z.t.Unix()
			if v < 0 && ns != 0 {
				v++
			}
			width = 0
			pad = ' '
		case 'U':
			v = int64((t.YearDay() + 6 - wd) / 7)
		case 'u':
			v = int64((wd+6)%7 + 1)
			width = 0
			pad = ' '
		case 'V':
			v = int64(iw)
		case 'W':
			v = int64((t.YearDay() + 6 - (wd+6)%7) / 7)
		case 'w':
			v = int64(wd)
			width = 0
			pad = ' '
		case 'Y':
			v = int64(y)
			width = 4
		case 'y':
			v = int64(y % 100)
		case 'z', 'Q':
			numeric = false
			applyCase = false
			if c == 'Q' && ext.colons > 1 || c == 'z' && ext.colons > 3 {
				return "", fmt.Errorf("invalid number of `:` in `%%%c` directive", c)
			}
			if c == 'Q' && z.tz.Name() != "" && !z.tz.unknown {
				value = z.tz.Name()
			} else {
				value = offsetString(off, ext.colons > 0, ext.colons < 3, ext.colons == 2)
			}
		case 'Z':
			numeric = false
			value = z.tz.info(z.t).abbreviation
			if z.tz.Name() == "" {
				value = offsetString(off, true, false, false)
			}
			if z.tz.unknown {
				value = "UTC"
			}
			if ext.flag == '#' {
				value = asciiLower(value)
			} else {
				value = asciiUpper(value)
			}
		case 'f', 'N', '.':
			numeric = false
			applyCase = false
			precision := ext.width
			dot := false
			if c == '.' {
				dot = true
				if i == len(s) {
					return "", errors.New("invalid format string, expected directive after `%.`")
				}
				precision, err = parseWidth(s, &i)
				if err != nil {
					return "", err
				}
				c = s[i]
				i++
				if c != 'f' {
					return "", fmt.Errorf("found unrecognized specifier directive `%s` following `%%.`", escapeByte(c))
				}
			}
			if !dot && precision == 0 {
				return "", fmt.Errorf("%%%c failed: zero precision with %%%c is not allowed", c, c)
			}
			if c == 'N' && precision < 0 {
				precision = 9
			}
			if dot && (precision == 0 || precision < 0 && ns == 0) {
				value = ""
			} else {
				value = fmt.Sprintf("%09d", ns)
				if precision < 0 {
					value = strings.TrimRight(value, "0")
					if value == "" {
						value = "0"
					}
				} else if precision < 9 {
					value = value[:precision]
				}
				if dot {
					value = "." + value
				}
			}
		case 'D', 'F', 'R', 'T', 'c', 'x', 'X', 'r':
			numeric = false
			applyCase = false
			sub := ""
			switch c {
			case 'D':
				sub = "%m/%d/%y"
			case 'F':
				ys := fmt.Sprintf("%04d", y)
				if y < 0 {
					ys = fmt.Sprintf("-%04d", -y)
				}
				value = fmt.Sprintf("%s-%02d-%02d", ys, month, day)
			case 'R':
				sub = "%H:%M"
			case 'T', 'X':
				sub = "%H:%M:%S"
			case 'c':
				sub = "%Y M%m %-d, %a %H:%M:%S"
				if ext.flag == '^' {
					sub = "%Y M%m %-d, %^a %H:%M:%S"
				}
			case 'x':
				sub = "%Y M%m %-d"
			case 'r':
				sub = "%-I:%M:%S %p"
			}
			if sub != "" {
				value, err = formatTime(sub, z)
				if err != nil {
					return "", fmt.Errorf("%%%c failed: %s", c, err)
				}
			}
		default:
			return "", fmt.Errorf("found unrecognized specifier directive `%s`", escapeByte(c))
		}
		if numeric {
			value = formatInteger(v, pad, width, ext)
		} else if applyCase && ext.flag == '^' {
			value = asciiUpper(value)
		}
		out.WriteString(value)
	}
	return out.String(), nil
}
func formatInteger(n int64, pad byte, width int, ext extension) string {
	if ext.flag == '0' {
		pad = '0'
	} else if ext.flag == '_' {
		pad = ' '
	}
	if ext.width >= 0 {
		width = ext.width
	}
	if ext.flag == '-' {
		width = 0
	}
	// Jiff's integer buffer caps positive widths at 20 digits. Negative
	// zero-padded numbers reserve the sign separately; space padding is
	// emitted directly and keeps the requested total width.
	if n >= 0 && width > 20 {
		width = 20
	}
	if n < 0 && pad == '0' && width > 21 {
		width = 21
	}
	s := strconv.FormatInt(n, 10)
	if len(s) >= width {
		return s
	}
	padding := strings.Repeat(string(pad), width-len(s))
	if n < 0 && pad == '0' {
		return "-" + padding + s[1:]
	}
	return padding + s
}
