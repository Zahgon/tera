// Ports the unit tests of tera-contrib/src/filesize_format.rs: test_filesizeformat_binary and
// test_filesizeformat_decimal; its test_register is part of TestRegister.

package contrib

import "testing"

func TestFilesizeFormatBinary(t *testing.T) {
	checkFilter(t, FilesizeFormat, 1024, nil, "1 KiB")
	checkFilter(t, FilesizeFormat, 1048576, nil, "1 MiB")
}
func TestFilesizeFormatDecimal(t *testing.T) {
	checkFilter(t, FilesizeFormat, 1000, map[string]any{"binary": false}, "1 kB")
}
