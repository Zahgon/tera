// Package slugify implements slug's ASCII slug construction with Latin transliteration.
package slugify

import "strings"

// Slugify returns lowercase words separated by a single hyphen. Latin-1 and
// Latin Extended-A letters are transliterated; other non-ASCII runes are omitted.
func Slugify(s string) string {
	var out strings.Builder
	dash := true
	push := func(c byte) {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			out.WriteByte(c)
			dash = false
		} else if !dash {
			out.WriteByte('-')
			dash = true
		}
	}
	for _, r := range s {
		if r < 128 {
			push(byte(r))
			continue
		}
		t := latin(r)
		for i := range t {
			push(t[i])
		}
	}
	return strings.TrimSuffix(out.String(), "-")
}

func latin(r rune) string {
	groups := []struct{ letters, base string }{
		{"ÀÁÂÃÄÅàáâãäåĀāĂăĄąª", "a"}, {"Ææ", "ae"}, {"ÇçĆćĈĉĊċČč", "c"},
		{"ÐðĎďĐđ", "d"}, {"ÈÉÊËèéêëĒēĔĕĖėĘęĚě", "e"}, {"ĜĝĞğĠġĢģ", "g"},
		{"ĤĥĦħ", "h"}, {"ÌÍÎÏìíîïĨĩĪīĬĭĮįİı", "i"}, {"Ĳĳ", "ij"}, {"Ĵĵ", "j"},
		{"Ķķĸ", "k"}, {"ĹĺĻļĽľĿŀŁł", "l"}, {"ÑñŃńŅņŇň", "n"},
		{"Ŋŋ", "ng"},
		{"µ", "u"},
		{"ÒÓÔÕÖØòóôõöøŌōŎŏŐőº", "o"}, {"Œœ", "oe"}, {"ŔŕŖŗŘř", "r"},
		{"ŚśŜŝŞşŠšſ", "s"}, {"ß", "ss"}, {"ŢţŤťŦŧ", "t"}, {"Þþ", "th"},
		{"ÙÚÛÜùúûüŨũŪūŬŭŮůŰűŲų", "u"}, {"Ŵŵ", "w"}, {"ÝýÿŶŷŸ", "y"}, {"ŹźŻżŽž", "z"},
		{"ŉ", "'n"},
	}
	for _, g := range groups {
		if strings.ContainsRune(g.letters, r) {
			return g.base
		}
	}
	return ""
}
