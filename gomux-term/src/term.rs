//! Alacritty-based Terminal with built-in PTY

use alacritty_terminal::{
    event::{Event, EventListener},
    grid::{Dimensions, Grid},
    index::{Column, Line},
    term::cell::Cell,
    tty::{Pty, Options as PtyOptions},
};
use std::ffi::OsStr;
use std::io::{Read, Write};
use std::os::raw::{c_char, c_int, c_uint};
use std::slice;

/// Event listener that just tracks if content changed
#[derive(Clone)]
struct ChangeTracker;

impl EventListener for ChangeTracker {
    fn send_event(&self, _event: Event) {}
}

/// Complete terminal with PTY and Alacritty grid
pub struct AlacrittyPane {
    pty: Pty,
    grid: Grid<Cell>,
    parser: alacritty_terminal::vte::ansi::Processor,
    needs_render: bool,
}

impl AlacrittyPane {
    fn new(rows: usize, cols: usize, shell: &str) -> Option<Self> {
        // Create PTY
        let options = PtyOptions {
            shell: Some(std::borrow::Cow::Borrowed(OsStr::new(shell))),
            working_directory: None,
            env: std::collections::HashMap::new(),
        };
        
        let pty = Pty::new(&options, rows, cols).ok()?;
        
        // Create grid
        let grid = Grid::new(rows, cols, 10000);
        let parser = alacritty_terminal::vte::ansi::Processor::new();
        
        Some(AlacrittyPane {
            pty,
            grid,
            parser,
            needs_render: false,
        })
    }

    fn read_and_process(&mut self) -> bool {
        let mut buf = [0u8; 4096];
        match self.pty.reader().read(&mut buf) {
            Ok(0) => false, // EOF
            Ok(n) => {
                // Feed bytes through parser
                self.parser.advance(self, &buf[..n]);
                self.needs_render = true;
                true
            }
            Err(_) => false,
        }
    }

    fn write_input(&mut self, data: &[u8]) {
        let _ = self.pty.writer().write_all(data);
    }

    fn get_line(&self, row: usize) -> String {
        if row >= self.grid.screen_lines() {
            return String::new();
        }
        let mut result = String::new();
        let line = Line(row as i32);
        for col in 0..self.grid.columns() {
            let cell = &self.grid[line][Column(col)];
            result.push(if cell.c == '\0' { ' ' } else { cell.c });
        }
        result
    }

    fn get_cursor(&self) -> (usize, usize) {
        let point = self.grid.cursor.point;
        (point.line.0 as usize, point.column.0 as usize)
    }

    fn mark_rendered(&mut self) {
        self.needs_render = false;
    }
}

// Implement Handler for the parser
impl alacritty_terminal::vte::ansi::Handler for AlacrittyPane {
    fn input(&mut self, c: char) {
        let point = self.grid.cursor.point;
        let max_lines = self.grid.screen_lines() as i32;
        
        if point.line.0 >= 0 && point.line.0 < max_lines {
            let mut cell = Cell::default();
            cell.c = c;
            self.grid[point.line][point.column] = cell;
            
            // Advance cursor
            let max_cols = self.grid.columns();
            if point.column.0 + 1 < max_cols {
                self.grid.cursor.point.column.0 += 1;
            } else {
                self.grid.cursor.point.column.0 = 0;
                self.grid.cursor.point.line.0 += 1;
            }
        }
    }

    fn linefeed(&mut self) {
        self.grid.cursor.point.line.0 += 1;
        // TODO: scroll if at bottom
    }

    fn carriage_return(&mut self) {
        self.grid.cursor.point.column.0 = 0;
    }

    fn backspace(&mut self) {
        if self.grid.cursor.point.column.0 > 0 {
            self.grid.cursor.point.column.0 -= 1;
        }
    }

    fn clear_screen(&mut self, _mode: alacritty_terminal::vte::ansi::ClearMode) {
        for row in 0..self.grid.screen_lines() {
            for col in 0..self.grid.columns() {
                self.grid[Line(row as i32)][Column(col)] = Cell::default();
            }
        }
        self.grid.cursor.point.line.0 = 0;
        self.grid.cursor.point.column.0 = 0;
    }

    fn goto(&mut self, line: i32, col: usize) {
        let max_line = (self.grid.screen_lines().saturating_sub(1) as i32).max(0);
        let max_col = self.grid.columns().saturating_sub(1);
        self.grid.cursor.point.line.0 = line.max(0).min(max_line);
        self.grid.cursor.point.column.0 = col.min(max_col);
    }

    // TODO: Add color handling, attributes, etc.
}

// FFI exports
use std::ffi::c_void;
use std::boxed::Box;

/// Create new pane with PTY running shell
#[no_mangle]
pub extern "C" fn gomux_pane_new(rows: c_uint, cols: c_uint, shell: *const c_char) -> *mut c_void {
    let shell_str = unsafe {
        if shell.is_null() {
            "/bin/sh"
        } else {
            std::ffi::CStr::from_ptr(shell).to_str().unwrap_or("/bin/sh")
        }
    };
    
    match AlacrittyPane::new(rows as usize, cols as usize, shell_str) {
        Some(pane) => Box::into_raw(Box::new(pane)) as *mut c_void,
        None => std::ptr::null_mut(),
    }
}

/// Free pane
#[no_mangle]
pub extern "C" fn gomux_pane_free(pane: *mut c_void) {
    if !pane.is_null() {
        unsafe {
            let _ = Box::from_raw(pane as *mut AlacrittyPane);
        }
    }
}

/// Read from PTY and update grid
/// Returns: 1 if content changed, 0 if no data, -1 if error
#[no_mangle]
pub extern "C" fn gomux_pane_tick(pane: *mut c_void) -> c_int {
    if pane.is_null() {
        return -1;
    }
    
    let pane = unsafe { &mut *(pane as *mut AlacrittyPane) };
    
    if pane.read_and_process() {
        1
    } else {
        0
    }
}

/// Write input to PTY
#[no_mangle]
pub extern "C" fn gomux_pane_write(pane: *mut c_void, data: *const c_char, len: c_uint) -> c_int {
    if pane.is_null() || data.is_null() {
        return -1;
    }
    
    let pane = unsafe { &mut *(pane as *mut AlacrittyPane) };
    let bytes = unsafe { slice::from_raw_parts(data as *const u8, len as usize) };
    
    pane.write_input(bytes);
    0
}

/// Get rendered line
#[no_mangle]
pub extern "C" fn gomux_pane_get_line(
    pane: *mut c_void,
    row: c_uint,
    buf: *mut c_char,
    max_len: c_uint,
) -> c_int {
    if pane.is_null() || buf.is_null() {
        return -1;
    }
    
    let pane = unsafe { &*(pane as *const AlacrittyPane) };
    let line = pane.get_line(row as usize);
    let bytes = line.as_bytes();
    let len = bytes.len().min(max_len as usize - 1);
    
    unsafe {
        std::ptr::copy_nonoverlapping(bytes.as_ptr() as *const c_char, buf, len);
        *buf.add(len) = 0;
    }
    
    len as c_int
}

/// Get cursor position
#[no_mangle]
pub extern "C" fn gomux_pane_get_cursor(
    pane: *mut c_void,
    row: *mut c_uint,
    col: *mut c_uint,
) -> c_int {
    if pane.is_null() || row.is_null() || col.is_null() {
        return -1;
    }
    
    let pane = unsafe { &*(pane as *const AlacrittyPane) };
    let (r, c) = pane.get_cursor();
    
    unsafe {
        *row = r as c_uint;
        *col = c as c_uint;
    }
    
    0
}

/// Check if needs render (content changed since last mark)
#[no_mangle]
pub extern "C" fn gomux_pane_needs_render(pane: *mut c_void) -> c_int {
    if pane.is_null() {
        return -1;
    }
    
    let pane = unsafe { &mut *(pane as *mut AlacrittyPane) };
    if pane.needs_render {
        1
    } else {
        0
    }
}

/// Mark as rendered
#[no_mangle]
pub extern "C" fn gomux_pane_mark_rendered(pane: *mut c_void) {
    if !pane.is_null() {
        let pane = unsafe { &mut *(pane as *mut AlacrittyPane) };
        pane.mark_rendered();
    }
}
