//! FFI wrapper for alacritty_terminal - Simplified Prototype
//! 
//! Exposes a C API for Go to use Alacritty's terminal emulator

use std::ffi::{c_char, c_void};
use std::os::raw::{c_int, c_uint};

/// Opaque handle to a terminal instance (stub for now)
pub struct TerminalHandle {
    rows: usize,
    cols: usize,
    grid: Vec<Vec<u8>>, // Simplified - just ASCII for now
    cursor_row: usize,
    cursor_col: usize,
}

/// Create a new terminal with PTY
/// 
/// Returns opaque pointer or null on error
#[no_mangle]
pub extern "C" fn gomux_term_new(rows: c_uint, cols: c_uint) -> *mut c_void {
    let rows = rows as usize;
    let cols = cols as usize;
    
    // Initialize empty grid
    let grid = vec![vec![b' '; cols]; rows];
    
    let handle = Box::new(TerminalHandle {
        rows,
        cols,
        grid,
        cursor_row: 0,
        cursor_col: 0,
    });
    
    Box::into_raw(handle) as *mut c_void
}

/// Free a terminal instance
#[no_mangle]
pub extern "C" fn gomux_term_free(handle: *mut c_void) {
    if !handle.is_null() {
        unsafe {
            Box::from_raw(handle as *mut TerminalHandle);
        }
    }
}

/// Process bytes from PTY (called when PTY has output)
/// Stub: just echoes bytes to grid at cursor position
#[no_mangle]
pub extern "C" fn gomux_term_process_pty(_handle: *mut c_void) -> c_int {
    // TODO: Integrate with real Alacritty terminal
    // For now, stub returns 0 (no data)
    0
}

/// Write bytes to PTY (user input)
/// Stub: writes to internal grid for display
#[no_mangle]
pub extern "C" fn gomux_term_write(handle: *mut c_void, data: *const c_char, len: c_uint) -> c_int {
    if handle.is_null() || data.is_null() {
        return -1;
    }
    
    let handle = unsafe { &mut *(handle as *mut TerminalHandle) };
    let slice = unsafe { std::slice::from_raw_parts(data as *const u8, len as usize) };
    
    for &byte in slice {
        match byte {
            b'\r' => {
                handle.cursor_col = 0;
            }
            b'\n' => {
                handle.cursor_row += 1;
                if handle.cursor_row >= handle.rows {
                    handle.cursor_row = handle.rows - 1;
                }
            }
            b'\x08' => {
                // Backspace
                if handle.cursor_col > 0 {
                    handle.cursor_col -= 1;
                    handle.grid[handle.cursor_row][handle.cursor_col] = b' ';
                }
            }
            0x1b => {
                // ESC - skip escape sequences (simplified)
                // Real impl would parse full sequences
            }
            32..=126 => {
                // Printable ASCII
                if handle.cursor_col < handle.cols {
                    handle.grid[handle.cursor_row][handle.cursor_col] = byte;
                    handle.cursor_col += 1;
                }
            }
            _ => {}
        }
    }
    
    0
}

/// Get a line from the grid (for rendering)
/// 
/// Copies up to max_len characters into buf, returns actual length
#[no_mangle]
pub extern "C" fn gomux_term_get_line(
    handle: *mut c_void,
    row: c_uint,
    buf: *mut c_char,
    max_len: c_uint,
) -> c_int {
    if handle.is_null() || buf.is_null() {
        return -1;
    }
    
    let handle = unsafe { &*(handle as *const TerminalHandle) };
    let row = row as usize;
    
    if row >= handle.rows {
        return 0;
    }
    
    let max_len = max_len as usize;
    let line = &handle.grid[row];
    let len = line.len().min(max_len - 1);
    
    unsafe {
        for i in 0..len {
            *buf.add(i) = line[i] as c_char;
        }
        *buf.add(len) = 0; // Null terminate
    }
    
    len as c_int
}

/// Get cursor position
#[no_mangle]
pub extern "C" fn gomux_term_get_cursor(
    handle: *mut c_void,
    row: *mut c_uint,
    col: *mut c_uint,
) -> c_int {
    if handle.is_null() || row.is_null() || col.is_null() {
        return -1;
    }
    
    let handle = unsafe { &*(handle as *const TerminalHandle) };
    
    unsafe {
        *row = handle.cursor_row as c_uint;
        *col = handle.cursor_col as c_uint;
    }
    
    0
}

/// Check if PTY has data available (for polling)
#[no_mangle]
pub extern "C" fn gomux_term_poll_pty(_handle: *mut c_void, _timeout_ms: c_int) -> c_int {
    // Stub: always returns ready
    1
}
