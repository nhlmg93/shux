/*
 * libtsm wrapper for gomux
 * 
 * Simple C wrapper around libtsm providing:
 * - Terminal state machine (VTE)
 * - Screen buffer (grid)
 * - No PTY (handled by Go)
 * - No rendering (handled by Go)
 */

#define _GNU_SOURCE
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <wchar.h>

// Include libtsm headers
#include "tsm/libtsm.h"
#include "tsm/libtsm-int.h"

// Our terminal structure
typedef struct {
    struct tsm_screen *screen;
    struct tsm_vte *vte;
    unsigned int rows;
    unsigned int cols;
    bool dirty;
    // Buffer for last draw
    char **lines;
} GomuxTerminal;

// Drawing callback to capture cell data
typedef struct {
    char **lines;
    unsigned int rows;
    unsigned int cols;
    unsigned int cursor_x;
    unsigned int cursor_y;
} DrawState;

static int draw_callback(struct tsm_screen *con,
                       struct tsm_screen_attr *attr,
                       tsm_char_t ch,
                       unsigned int x,
                       unsigned int y,
                       tsm_age_t age,
                       void *data) {
    (void)con;
    (void)attr;
    (void)age;
    
    DrawState *state = (DrawState*)data;
    if (!state || y >= state->rows || x >= state->cols) return 0;
    
    // Convert wchar_t to char (simplified - proper Unicode handling needed)
    if (ch < 128 && ch > 0) {
        state->lines[y][x] = (char)ch;
    } else if (ch == 0) {
        state->lines[y][x] = ' ';
    } else {
        state->lines[y][x] = '?'; // Non-ASCII placeholder
    }
    
    return 0;
}

// Create new terminal
void* gomux_term_new(unsigned int rows, unsigned int cols) {
    GomuxTerminal *term = calloc(1, sizeof(GomuxTerminal));
    if (!term) return NULL;
    
    term->rows = rows;
    term->cols = cols;
    term->dirty = true;
    
    // Allocate line buffers
    term->lines = calloc(rows, sizeof(char*));
    for (unsigned int i = 0; i < rows; i++) {
        term->lines[i] = calloc(cols + 1, sizeof(char));
        memset(term->lines[i], ' ', cols);
        term->lines[i][cols] = '\0';
    }
    
    // Create screen
    if (tsm_screen_new(&term->screen, NULL, NULL) < 0) {
        goto fail;
    }
    
    // Resize screen to our dimensions
    tsm_screen_resize(term->screen, cols, rows);
    
    // Create VTE state machine attached to screen
    if (tsm_vte_new(&term->vte, term->screen, NULL, NULL, NULL, NULL) < 0) {
        goto fail;
    }
    
    return term;
    
fail:
    if (term->lines) {
        for (unsigned int i = 0; i < rows; i++) {
            free(term->lines[i]);
        }
        free(term->lines);
    }
    if (term->screen) tsm_screen_unref(term->screen);
    free(term);
    return NULL;
}

// Free terminal
void gomux_term_free(void* handle) {
    if (!handle) return;
    
    GomuxTerminal *term = (GomuxTerminal*)handle;
    if (term->vte) tsm_vte_unref(term->vte);
    if (term->screen) tsm_screen_unref(term->screen);
    
    if (term->lines) {
        for (unsigned int i = 0; i < term->rows; i++) {
            free(term->lines[i]);
        }
        free(term->lines);
    }
    
    free(term);
}

// Update our line buffers from screen
static void update_lines(GomuxTerminal *term) {
    if (!term || !term->screen || !term->lines) return;
    
    // Clear lines first
    for (unsigned int i = 0; i < term->rows; i++) {
        memset(term->lines[i], ' ', term->cols);
        term->lines[i][term->cols] = '\0';
    }
    
    // Draw screen to our buffers
    DrawState state = {
        .lines = term->lines,
        .rows = term->rows,
        .cols = term->cols,
        .cursor_x = 0,
        .cursor_y = 0
    };
    
    tsm_screen_draw(term->screen, draw_callback, &state);
}

// Process bytes through VTE
void gomux_term_process(void* handle, const char* data, size_t len) {
    if (!handle || !data || len == 0) return;
    
    GomuxTerminal *term = (GomuxTerminal*)handle;
    tsm_vte_input(term->vte, data, len);
    term->dirty = true;
}

// Get line content (for compatibility with old API)
int gomux_term_get_line_wrapped(void* handle, unsigned int row, char* buf, unsigned int max_len) {
    if (!handle || !buf || max_len == 0) return -1;
    
    GomuxTerminal *term = (GomuxTerminal*)handle;
    if (row >= term->rows) {
        buf[0] = '\0';
        return 0;
    }
    
    // Update lines from screen
    update_lines(term);
    
    // Copy line to output buffer
    unsigned int to_copy = term->cols;
    if (to_copy > max_len - 1) {
        to_copy = max_len - 1;
    }
    
    memcpy(buf, term->lines[row], to_copy);
    buf[to_copy] = '\0';
    
    return to_copy;
}

// Get cursor position
int gomux_term_get_cursor(void* handle, unsigned int* row, unsigned int* col) {
    if (!handle || !row || !col) return -1;
    
    GomuxTerminal *term = (GomuxTerminal*)handle;
    
    // Access cursor position from screen internals
    if (term->screen) {
        // libtsm stores cursor in screen structure
        // We need to access it through the internal header
        *col = term->screen->cursor_x;
        *row = term->screen->cursor_y;
        return 0;
    }
    
    *row = 0;
    *col = 0;
    return -1;
}

// Resize terminal
void gomux_term_resize(void* handle, unsigned int rows, unsigned int cols) {
    if (!handle) return;
    
    GomuxTerminal *term = (GomuxTerminal*)handle;
    
    // Free old lines
    if (term->lines) {
        for (unsigned int i = 0; i < term->rows; i++) {
            free(term->lines[i]);
        }
        free(term->lines);
    }
    
    // Update dimensions
    term->rows = rows;
    term->cols = cols;
    term->dirty = true;
    
    // Allocate new lines
    term->lines = calloc(rows, sizeof(char*));
    for (unsigned int i = 0; i < rows; i++) {
        term->lines[i] = calloc(cols + 1, sizeof(char));
        memset(term->lines[i], ' ', cols);
        term->lines[i][cols] = '\0';
    }
    
    // Resize screen
    if (term->screen) {
        tsm_screen_resize(term->screen, cols, rows);
    }
}

// Get dimensions
unsigned int gomux_term_get_rows(void* handle) {
    if (!handle) return 0;
    return ((GomuxTerminal*)handle)->rows;
}

unsigned int gomux_term_get_cols(void* handle) {
    if (!handle) return 0;
    return ((GomuxTerminal*)handle)->cols;
}

// Dirty flag
int gomux_term_needs_render(void* handle) {
    if (!handle) return -1;
    return ((GomuxTerminal*)handle)->dirty ? 1 : 0;
}

void gomux_term_mark_rendered(void* handle) {
    if (!handle) return;
    ((GomuxTerminal*)handle)->dirty = false;
}

int gomux_term_noop(void* handle) {
    (void)handle;
    return 0;
}
