// Code generated from Rust char classification; DO NOT EDIT.
// regex-syntax uses the toolchain Unicode tables for capture names.
package regex

var nameAlphaAdded = runeSet{
	{0x88f, 0x88f},
	{0xc5c, 0xc5c},
	{0xcdc, 0xcdc},
	{0xa7ce, 0xa7cf},
	{0xa7d2, 0xa7d2},
	{0xa7d4, 0xa7d4},
	{0xa7f1, 0xa7f1},
	{0x10940, 0x10959},
	{0x10ec5, 0x10ec7},
	{0x10efa, 0x10efb},
	{0x11b60, 0x11b67},
	{0x11db0, 0x11ddb},
	{0x16ea0, 0x16eb8},
	{0x16ebb, 0x16ed3},
	{0x16ff2, 0x16ff6},
	{0x187f8, 0x187ff},
	{0x18d09, 0x18d1e},
	{0x18d80, 0x18df2},
	{0x1e6c0, 0x1e6de},
	{0x1e6e0, 0x1e6f5},
	{0x1e6fe, 0x1e6ff},
	{0x2b73a, 0x2b73f},
	{0x2cea2, 0x2cead},
	{0x323b0, 0x33479},
}
var nameAlphaRemoved = runeSet{}
var nameNumberAdded = runeSet{
	{0x11de0, 0x11de9},
	{0x16ff4, 0x16ff6},
}
var nameNumberRemoved = runeSet{}

func nameAlphabetic(r rune) bool {
	base, _ := unicodeData("property_bool:alphabetic")
	return (inSet(base, r) || inSet(nameAlphaAdded, r)) && !inSet(nameAlphaRemoved, r)
}
func nameNumeric(r rune) bool {
	base, _ := unicodeData("general_category:number")
	return (inSet(base, r) || inSet(nameNumberAdded, r)) && !inSet(nameNumberRemoved, r)
}
