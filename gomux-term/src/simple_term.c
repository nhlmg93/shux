/*
 * Simple terminal emulator in C
 * 
 * Minimal VT100 emulator for gomux
 * - No dependencies
 * - Basic escape sequences (cursor, colors, clear)
 * - Grid-based screen buffer
 */

#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <ctype.h>
#include <stdio.h>

#define MAX_COLS 512
#define MAX_ROWS 256

// Cell attributes
typedef struct {
    char ch;           // Character (simplified - no Unicode)
    unsigned char fg;  // Foreground color (0-15)
    unsigned char bg;  // Background color (0-15)
    unsigned char attrs; // Bold, underline, etc.
} Cell;

// Terminal state
typedef struct {
    Cell grid[MAX_ROWS][MAX_COLS];
    unsigned int rows;
    unsigned int cols;
    unsigned int cursor_row;
    unsigned int cursor_col;
    
    // Escape sequence state
    bool in_escape;
    char escape_buf[32];
    int escape_len;
    
    // Attributes
    unsigned char cur_fg;
    unsigned char cur_bg;
    bool dirty;
} Terminal;

// Create terminal
void* gomux_term_new(unsigned int rows, unsigned int cols) {
    if (rows > MAX_ROWS) rows = MAX_ROWS;
    if (cols > MAX_COLS) cols = MAX_COLS;
    
    Terminal *term = calloc(1, sizeof(Terminal));
    if (!term) return NULL;
    
    term->rows = rows;
    term->cols = cols;
    term->cursor_row = 0;
    term->cursor_col = 0;
    term->cur_fg = 7;  // White
    term->cur_bg = 0;  // Black
    term->dirty = true;
    term->in_escape = false;
    
    // Clear grid
    for (unsigned int r = 0; r < rows; r++) {
        for (unsigned int c = 0; c < cols; c++) {
            term->grid[r][c].ch = ' ';
            term->grid[r][c].fg = 7;
            term->grid[r][c].bg = 0;
        }
    }
    
    return term;
}

// Free terminal
void gomux_term_free(void* handle) {
    free(handle);
}

// Clear screen
static void clear_screen(Terminal *term) {
    for (unsigned int r = 0; r < term->rows; r++) {
        for (unsigned int c = 0; c < term->cols; c++) {
            term->grid[r][c].ch = ' ';
            term->grid[r][c].fg = term->cur_fg;
            term->grid[r][c].bg = term->cur_bg;
        }
    }
    term->cursor_row = 0;
    term->cursor_col = 0;
}

// Move cursor
static void move_cursor(Terminal *term, int row, int col) {
    if (row < 0) row = 0;
    if (col < 0) col = 0;
    if ((unsigned int)row >= term->rows) row = term->rows - 1;
    if ((unsigned int)col >= term->cols) col = term->cols - 1;
    
    term->cursor_row = row;
    term->cursor_col = col;
}

// Handle escape sequence
static void handle_escape(Terminal *term) {
    char *seq = term->escape_buf;
    
    if (seq[0] == '[') {
        // CSI sequence
        int params[4] = {0, 0, 0, 0};
        int pcount = 0;
        char cmd = 0;
        
        // Parse parameters
        char *p = seq + 1;
        while (*p && pcount < 4) {
            if (isdigit(*p)) {
                params[pcount] = params[pcount] * 10 + (*p - '0');
            } else if (*p == ';') {
                pcount++;
            } else if (isalpha(*p)) {
                cmd = *p;
                break;
            }
            p++;
        }
        
        switch (cmd) {
            case 'H':  // Cursor position
            case 'f':
                move_cursor(term, params[0] ? params[0] - 1 : 0, 
                                  params[1] ? params[1] - 1 : 0);
                break;
            case 'A':  // Up
                move_cursor(term, term->cursor_row - (params[0] ? params[0] : 1), term->cursor_col);
                break;
            case 'B':  // Down
                move_cursor(term, term->cursor_row + (params[0] ? params[0] : 1), term->cursor_col);
                break;
            case 'C':  // Right
                move_cursor(term, term->cursor_row, term->cursor_col + (params[0] ? params[0] : 1));
                break;
            case 'D':  // Left
                move_cursor(term, term->cursor_row, term->cursor_col - (params[0] ? params[0] : 1));
                break;
            case 'J':  // Erase display
                if (params[0] == 2) {
                    clear_screen(term);
                }
                break;
            case 'K':  // Erase line
                if (params[0] == 0 || params[0] == 2) {
                    // Clear from cursor to end of line
                    for (unsigned int c = term->cursor_col; c < term->cols; c++) {
                        term->grid[term->cursor_row][c].ch = ' ';
                    }
                }
                if (params[0] == 1 || params[0] == 2) {
                    // Clear from start to cursor
                    for (unsigned int c = 0; c <= term->cursor_col; c++) {
                        term->grid[term->cursor_row][c].ch = ' ';
                    }
                }
                break;
            case 'm':  // Set attributes (colors)
                // Simplified: just handle basic colors
                for (int i = 0; i <= pcount; i++) {
                    if (params[i] == 0) {  // Reset
                        term->cur_fg = 7;
                        term->cur_bg = 0;
                    } else if (params[i] >= 30 && params[i] <= 37) {  // Foreground
                        term->cur_fg = params[i] - 30;
                    } else if (params[i] >= 40 && params[i] <= 47) {  // Background
                        term->cur_bg = params[i] - 40;
                    }
                }
                break;
        }
    }
}

// Process single character
static void process_char(Terminal *term, char c) {
    if (term->in_escape) {
        if (term->escape_len < 31) {
            term->escape_buf[term->escape_len++] = c;
            term->escape_buf[term->escape_len] = '\0';
        }
        
        // Check for end of escape sequence
        if (isalpha(c) || c == '~') {
            handle_escape(term);
            term->in_escape = false;
            term->escape_len = 0;
        } else if (c == 'c') {  // Reset
            clear_screen(term);
            term->in_escape = false;
            term->escape_len = 0;
        }
        return;
    }
    
    switch (c) {
        case '\x1b':  // ESC
            term->in_escape = true;
            term->escape_len = 0;
            term->escape_buf[0] = '\0';
            break;
        case '\r':
            term->cursor_col = 0;
            break;
        case '\n':
            term->cursor_row++;
            if (term->cursor_row >= term->rows) {
                // Scroll: move everything up
                memmove(&term->grid[0], &term->grid[1], 
                       (term->rows - 1) * MAX_COLS * sizeof(Cell));
                // Clear last line
                for (unsigned int c = 0; c < term->cols; c++) {
                    term->grid[term->rows - 1][c].ch = ' ';
                }
                term->cursor_row = term->rows - 1;
            }
            break;
        case '\t':
            // Tab to next 8-column boundary
            term->cursor_col = ((term->cursor_col / 8) + 1) * 8;
            if (term->cursor_col >= term->cols) {
                term->cursor_col = term->cols - 1;
            }
            break;
        case '\b':  // Backspace
            if (term->cursor_col > 0) {
                term->cursor_col--;
            }
            break;
        default:
            if (c >= 32 && c < 127) {
                // Printable character
                if (term->cursor_row < term->rows && term->cursor_col < term->cols) {
                    term->grid[term->cursor_row][term->cursor_col].ch = c;
                    term->grid[term->cursor_row][term->cursor_col].fg = term->cur_fg;
                    term->grid[term->cursor_row][term->cursor_col].bg = term->cur_bg;
                    term->cursor_col++;
                    if (term->cursor_col >= term->cols) {
                        term->cursor_col = 0;
                        term->cursor_row++;
                        if (term->cursor_row >= term->rows) {
                            // Scroll
                            memmove(&term->grid[0], &term->grid[1], 
                                   (term->rows - 1) * MAX_COLS * sizeof(Cell));
                            for (unsigned int col = 0; col < term->cols; col++) {
                                term->grid[term->rows - 1][col].ch = ' ';
                            }
                            term->cursor_row = term->rows - 1;
                        }
                    }
                }
            }
            break;
    }
    
    term->dirty = true;
}

// Process bytes
void gomux_term_process(void* handle, const char* data, size_t len) {
    if (!handle || !data) return;
    
    Terminal *term = (Terminal*)handle;
    for (size_t i = 0; i < len; i++) {
        process_char(term, data[i]);
    }
}

// Get line content
int gomux_term_get_line_wrapped(void* handle, unsigned int row, char* buf, unsigned int max_len) {
    if (!handle || !buf || max_len == 0) return -1;
    
    Terminal *term = (Terminal*)handle;
    if (row >= term->rows) {
        buf[0] = '\0';
        return 0;
    }
    
    unsigned int to_copy = term->cols;
    if (to_copy > max_len - 1) {
        to_copy = max_len - 1;
    }
    
    for (unsigned int i = 0; i < to_copy; i++) {
        buf[i] = term->grid[row][i].ch;
    }
    buf[to_copy] = '\0';
    
    return to_copy;
}

// Get cursor position
int gomux_term_get_cursor(void* handle, unsigned int* row, unsigned int* col) {
    if (!handle || !row || !col) return -1;
    
    Terminal *term = (Terminal*)handle;
    *row = term->cursor_row;
    *col = term->cursor_col;
    return 0;
}

// Resize terminal
void gomux_term_resize(void* handle, unsigned int rows, unsigned int cols) {
    if (!handle) return;
    
    Terminal *term = (Terminal*)handle;
    if (rows > MAX_ROWS) rows = MAX_ROWS;
    if (cols > MAX_COLS) cols = MAX_COLS;
    
    // For simplicity, just update dimensions (don't resize content)
    term->rows = rows;
    term->cols = cols;
    
    // Ensure cursor is in bounds
    if (term->cursor_row >= rows) term->cursor_row = rows - 1;
    if (term->cursor_col >= cols) term->cursor_col = cols - 1;
    
    term->dirty = true;
}

// Get dimensions
unsigned int gomux_term_get_rows(void* handle) {
    if (!handle) return 0;
    return ((Terminal*)handle)->rows;
}

unsigned int gomux_term_get_cols(void* handle) {
    if (!handle) return 0;
    return ((Terminal*)handle)->cols;
}

// Dirty flag
int gomux_term_needs_render(void* handle) {
    if (!handle) return -1;
    return ((Terminal*)handle)->dirty ? 1 : 0;
}

void gomux_term_mark_rendered(void* handle) {
    if (!handle) return;
    ((Terminal*)handle)->dirty = false;
}

int gomux_term_noop(void* handle) {
    (void)handle;
    return 0;
}
