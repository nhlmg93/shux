package gomux

/*
#cgo LDFLAGS: -L${SRCDIR}/../../gomux-term/target/release -lgomux_term -lpthread -ldl
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

// NewAlacrittyTerm creates a new terminal
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

// ProcessBytes feeds bytes through the terminal emulator
func (t *AlacrittyTerm) ProcessBytes(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	ret := C.gomux_term_process_bytes(t.ptr, (*C.char)(unsafe.Pointer(&data[0])), C.uint(len(data)))
	if ret != 0 {
		return fmt.Errorf("process bytes failed")
	}
	return nil
}

// GetLine returns a row from the grid
func (t *AlacrittyTerm) GetLine(row int) string {
	buf := make([]byte, 1024)
	n := C.gomux_term_get_line(t.ptr, C.uint(row), (*C.char)(unsafe.Pointer(&buf[0])), 1024)
	if n <= 0 {
		return ""
	}
	return string(buf[:n])
}

// GetCursor returns cursor position
func (t *AlacrittyTerm) GetCursor() (row, col int) {
	var r, c C.uint
	C.gomux_term_get_cursor(t.ptr, &r, &c)
	return int(r), int(c)
}

// Write sends input (for API compatibility, uses ProcessBytes internally)
func (t *AlacrittyTerm) Write(data []byte) error {
	return t.ProcessBytes(data)
}

// Render returns full terminal content
func (t *AlacrittyTerm) Render(rows int) string {
	var result string
	for i := 0; i < rows; i++ {
		result += t.GetLine(i) + "\n"
	}
	return result
}
