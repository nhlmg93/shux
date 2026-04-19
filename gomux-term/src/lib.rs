//! FFI wrapper for alacritty_terminal - Simplified
//! 
//! Uses Alacritty's Grid for screen buffer, manual escape sequence handling

use alacritty_terminal::{
    grid::{Dimensions, Grid},
    index::{Column, Line},
    term::cell::{Cell, Flags},
};
use std::ffi::{c_char, c_void};
use std::os::raw::{c_int, c_uint};

/// Terminal handle with Alacritty Grid
pub struct TerminalHandle {
    grid: Grid<Cell>,
    cursor_row: usize,
    cursor_col: usize,
    in_escape: bool,
    escape_buf: Vec<u8>,
}

impl TerminalHandle {
    fn new(rows: usize, cols: usize) -> Self {
        TerminalHandle {
            grid: Grid::new(rows, cols, 10000),
            cursor_row: 0,
            cursor_col: 0,
            in_escape: false,
            escape_buf: Vec::new(),
        }
    }

    fn process_byte(&mut self, b: u8) {
        if self.in_escape {
            self.escape_buf.push(b);
            // Check if escape sequence is complete
            if (b >= b'A' && b <= b'Z') || (b >= b'a' && b <= b'z') || b == 0x07 {
                self.handle_escape();
                self.in_escape = false;
                self.escape_buf.clear();
            }
            return;
        }

        match b {
            b'\r' => self.cursor_col = 0,
            b'\n' => self.linefeed(),
            0x08 => self.backspace(), // BS
            0x7f => self.backspace(), // DEL
            0x03 => {} // Ctrl+C - ignore visually
            0x0c => self.clear_screen(), // Ctrl+L / Form feed
            0x1b => {
                // ESC - start escape sequence
                self.in_escape = true;
                self.escape_buf.push(b);
            }
            32..=126 => {
                // Printable ASCII
                self.write_char(b as char);
            }
            _ => {} // Ignore other control chars
        }
    }

    fn write_char(&mut self, c: char) {
        if self.cursor_row < self.grid.screen_lines() && self.cursor_col < self.grid.columns() {
            let mut cell = Cell::default();
            cell.c = c;
            self.grid[Line(self.cursor_row as i32)][Column(self.cursor_col)] = cell;
            self.cursor_col += 1;
            if self.cursor_col >= self.grid.columns() {
                self.cursor_col = 0;
                self.linefeed();
            }
        }
    }

    fn linefeed(&mut self) {
        self.cursor_row += 1;
        if self.cursor_row >= self.grid.screen_lines() {
            // Scroll up - clear top line
            for col in 0..self.grid.columns() {
                self.grid[Line(0)][Column(col)] = Cell::default();
            }
            self.cursor_row = self.grid.screen_lines() - 1;
        }
    }

    fn backspace(&mut self) {
        if self.cursor_col > 0 {
            self.cursor_col -= 1;
            if self.cursor_row < self.grid.screen_lines() && self.cursor_col < self.grid.columns() {
                self.grid[Line(self.cursor_row as i32)][Column(self.cursor_col)] = Cell::default();
            }
        }
    }

    fn clear_screen(&mut self) {
        for row in 0..self.grid.screen_lines() {
            for col in 0..self.grid.columns() {
                self.grid[Line(row as i32)][Column(col)] = Cell::default();
            }
        }
        self.cursor_row = 0;
        self.cursor_col = 0;
    }

    fn handle_escape(&mut self) {
        // Handle common escape sequences
        if self.escape_buf.len() < 2 {
            return;
        }

        let seq = &self.escape_buf[1..]; // Skip ESC
        
        match seq {
            [b'[', b'2', b'J'] => self.clear_screen(), // Clear entire screen
            [b'[', b'H'] | [b'[', b'f'] => {
                // Home cursor
                self.cursor_row = 0;
                self.cursor_col = 0;
            }
            [b'[', b'K'] => {
                // Clear to end of line
                for col in self.cursor_col..self.grid.columns() {
                    self.grid[Line(self.cursor_row as i32)][Column(col)] = Cell::default();
                }
            }
            _ => {
                // CSI sequences with numbers: ESC [ row ; col H (cursor position)
                if seq.len() > 2 && seq[0] == b'[' && seq.last().map(|&c| c == b'H' || c == b'f').unwrap_or(false) {
                    // Parse cursor position
                    let params: Vec<usize> = seq[1..seq.len()-1]
                        .split(|&b| b == b';')
                        .filter_map(|p| std::str::from_utf8(p).ok()?.parse().ok())
                        .collect();
                    
                    if params.len() >= 2 {
                        // ANSI coords are 1-based
                        self.cursor_row = (params[0].saturating_sub(1)).min(self.grid.screen_lines() - 1);
                        self.cursor_col = (params[1].saturating_sub(1)).min(self.grid.columns() - 1);
                    }
                }
            }
        }
    }

    fn get_line(&self, row: usize) -> String {
        if row >= self.grid.screen_lines() {
            return String::new();
        }
        let mut result = String::new();
        for col in 0..self.grid.columns() {
            let cell = &self.grid[Line(row as i32)][Column(col)];
            if cell.c == '\0' || cell.c == ' ' {
                result.push(' ');
            } else {
                result.push(cell.c);
            }
        }
        result
    }
}

/// Create new terminal
#[no_mangle]
pub extern "C" fn gomux_term_new(rows: c_uint, cols: c_uint) -> *mut c_void {
    let handle = Box::new(TerminalHandle::new(rows as usize, cols as usize));
    Box::into_raw(handle) as *mut c_void
}

/// Free terminal
#[no_mangle]
pub extern "C" fn gomux_term_free(handle: *mut c_void) {
    if !handle.is_null() {
        unsafe {
            let _ = Box::from_raw(handle as *mut TerminalHandle);
        }
    }
}

/// Process bytes
#[no_mangle]
pub extern "C" fn gomux_term_process_bytes(handle: *mut c_void, data: *const c_char, len: c_uint) -> c_int {
    if handle.is_null() || data.is_null() {
        return -1;
    }
    
    let handle = unsafe { &mut *(handle as *mut TerminalHandle) };
    let bytes = unsafe { std::slice::from_raw_parts(data as *const u8, len as usize) };
    
    for &b in bytes {
        handle.process_byte(b);
    }
    
    0
}

/// Get line (simplified - just ASCII)
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
    let line = handle.get_line(row as usize);
    let bytes = line.as_bytes();
    let len = bytes.len().min(max_len as usize - 1);
    
    unsafe {
        std::ptr::copy_nonoverlapping(bytes.as_ptr() as *const c_char, buf, len);
        *buf.add(len) = 0;
    }
    
    len as c_int
}

/// Get cursor
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

/// Stubs for compatibility
#[no_mangle]
pub extern "C" fn gomux_term_write(_handle: *mut c_void, _data: *const c_char, _len: c_uint) -> c_int {
    0
}

#[no_mangle]
pub extern "C" fn gomux_term_poll_pty(_handle: *mut c_void, _timeout_ms: c_int) -> c_int {
    1
}

#[no_mangle]
pub extern "C" fn gomux_term_process_pty(_handle: *mut c_void) -> c_int {
    0
}
