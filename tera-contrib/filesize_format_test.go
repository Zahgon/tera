package contrib

import "testing"

func TestFilesizeFormatBinary(t *testing.T) {
	checkFilter(t, FilesizeFormat, 1024, nil, "1 KiB")
	checkFilter(t, FilesizeFormat, 1048576, nil, "1 MiB")
}
func TestFilesizeFormatDecimal(t *testing.T) {
	checkFilter(t, FilesizeFormat, 1000, map[string]any{"binary": false}, "1 kB")
}
