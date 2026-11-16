package tera

import (
	"errors"
	"fmt"
	"io"
	"syscall"

	"github.com/Keats/tera/internal/rustrt"
)

type ioCause string

// Error returns the error message.
func (e ioCause) Error() string { return string(e) }
func rustIOError(e error) error {
	pairs := []struct {
		errno syscall.Errno
		text  string
	}{{syscall.ENOENT, "No such file or directory"}, {syscall.EACCES, "Permission denied"}, {syscall.EPERM, "Operation not permitted"}, {syscall.EISDIR, "Is a directory"}, {syscall.ENOTDIR, "Not a directory"}, {syscall.EPIPE, "Broken pipe"}, {syscall.EEXIST, "File exists"}, {syscall.ENOSPC, "No space left on device"}}
	for _, p := range pairs {
		if errors.Is(e, p.errno) {
			return ioCause(fmt.Sprintf("%s (os error %d)", p.text, p.errno))
		}
	}
	return e
}
func ioError(e error) *Error {
	k := "Other"
	if _, ok := e.(writeZeroError); ok {
		return &Error{Kind: KindIo("WriteZero"), Source: e}
	}
	if errors.Is(e, io.ErrShortWrite) {
		return &Error{Kind: KindIo("WriteZero"), Source: writeZeroError{}}
	}
	switch {
	case errors.Is(e, syscall.EPIPE):
		k = "BrokenPipe"
	case errors.Is(e, syscall.ENOENT):
		k = "NotFound"
	case errors.Is(e, syscall.EACCES):
		k = "PermissionDenied"
	case errors.Is(e, syscall.EINVAL):
		k = "InvalidInput"
	case errors.Is(e, syscall.ENOSPC):
		k = "StorageFull"
	}
	return &Error{Kind: KindIo(k), Source: rustIOError(e)}
}

func validateRendered(text string) error {
	if msg := rustrt.UTF8Error([]byte(text)); msg != "" {
		return &Error{Kind: KindUtf8Conversion{}, Source: ioCause(msg)}
	}
	return nil
}

type writeZeroError struct{}

// Error returns the write-zero diagnostic.
func (writeZeroError) Error() string { return "failed to write whole buffer" }
func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, e := w.Write(data)
		if n < 0 || n > len(data) {
			return ioCause("invalid write count")
		}
		if e != nil {
			return e
		}
		if n == 0 {
			return writeZeroError{}
		}
		data = data[n:]
	}
	return nil
}
