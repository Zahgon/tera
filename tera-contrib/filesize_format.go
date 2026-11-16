package contrib

import (
	"github.com/Keats/tera/tera"
	"github.com/Keats/tera/tera-contrib/internal/humansize"
)

// FilesizeFormat formats a number of bytes as a human-readable size. It uses binary units (KiB, MiB, ...) unless
// the `binary` argument is false, in which case decimal units (kB, MB, ...) are used.
//
//	{{ num_bytes | filesize_format }}
//	{{ num_bytes | filesize_format(binary=false) }}
var FilesizeFormat = tera.TypedFilter(tera.ArgU64, func(val uint64, kwargs tera.Kwargs, _ *tera.State) (tera.Value, error) {
	binary, ok, err := tera.KwargsGet(kwargs, "binary", tera.ArgBool)
	if err != nil {
		return tera.Undefined(), err
	}
	if !ok {
		binary = true
	}
	return tera.NormalString(humansize.FormatSize(val, binary)), nil
})
