package gomux

/*
#cgo LDFLAGS: -L${SRCDIR}/../../gomux-term/target/release -lgomux_term
#include "../../gomux-term/gomux_term.h"
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// AlacrittyTerm wraps the FFI terminal
type AlacrittyTerm struct {
	ptr C.GomuxTerm
}

// NewAlacrittyTerm creates a new terminal with PTY
func NewAlacrittyTerm(rows, cols int) (*AlacrittyTerm, error) {
	ptr := C.gomux_term_new(C.uint(rows), C.uint(cols))
	if ptr == nil {
		return nil, fmt.Errorf("failed to create terminal")
	}
	return &AlacrittyTerm{ptr: ptr}, nil
}

// Close frees the terminal
func (t *AlacrittyTerm) Close() {
	if t.ptr != nil {
		C.gomux_term_free(t.ptr)
		t.ptr = nil
	}
}

// ProcessPTY reads from PTY and updates internal grid
func (t *AlacrittyTerm) ProcessPTY() (int, error) {
	n := C.gomux_term_process_pty(t.ptr)
	if n < 0 {
		return 0, fmt.Errorf("PTY error")
	}
	return int(n), nil
}

// Write sends input to PTY
func (t *AlacrittyTerm) Write(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	ret := C.gomux_term_write(t.ptr, (*C.char)(unsafe.Pointer(&data[0])), C.uint(len(data)))
	if ret != 0 {
		return fmt.Errorf("write failed")
	}
	return nil
}

// GetLine returns a row from the grid for rendering
func (t *AlacrittyTerm) GetLine(row int) string {
	buf := make([]byte, 1024)
	n := C.gomux_term_get_line(t.ptr, C.uint(row), (*C.char)(unsafe.Pointer(&buf[0])), 1024)
	if n <= 0 {
		return ""
	}
	return string(buf[:n])
}

// GetCursor returns cursor position
func (t *AlacrittyTerm) GetCursor() (row, col int, err error) {
	var r, c C.uint
	ret := C.gomux_term_get_cursor(t.ptr, &r, &c)
	if ret != 0 {
		return 0, 0, fmt.Errorf("failed to get cursor")
	}
	return int(r), int(c), nil
}

// PollPTY checks if PTY has data (non-blocking with timeout)
func (t *AlacrittyTerm) PollPTY(timeoutMs int) (bool, error) {
	ret := C.gomux_term_poll_pty(t.ptr, C.int(timeoutMs))
	if ret < 0 {
		return false, fmt.Errorf("poll error")
	}
	return ret > 0, nil
}
