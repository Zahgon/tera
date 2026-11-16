// Package humansize formats byte counts using humansize's BINARY and DECIMAL defaults.
package humansize

import (
	"math"
	"strconv"
)

// FormatSize formats n with a space and a binary or decimal abbreviated unit.
func FormatSize(n uint64, binary bool) string {
	units := [...]string{"B", "kB", "MB", "GB", "TB", "PB", "EB"}
	divisor := 1000.0
	if binary {
		divisor = 1024
		units = [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	}
	f := float64(n)
	i := 0
	for f >= divisor && i < len(units)-1 {
		f /= divisor
		i++
	}
	p := 2
	if f-math.Trunc(f) <= math.Nextafter(1, 2)-1 {
		p = 0
	}
	return strconv.FormatFloat(f, 'f', p, 64) + " " + units[i]
}
