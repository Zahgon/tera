// Ports the unit test of tera/src/errors.rs: test_error_is_send_and_sync (an Error shared between goroutines).

package tera

import (
	"sync"
	"testing"
)

func TestErrorIsSendAndSync(t *testing.T) {
	// Go values can cross goroutines. Exercise immutable error sharing concurrently,
	// the property asserted by Rust's Send + Sync compile-time test.
	err := ErrorMessage("shared error")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err.Error() != "shared error" {
				t.Error("shared error changed")
			}
		}()
	}
	wg.Wait()
}
