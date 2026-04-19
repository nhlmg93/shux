#ifndef GOMUX_TERM_H
#define GOMUX_TERM_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Opaque handle to Terminal (libtsm wrapper)
typedef void* GomuxTerm;

// Create new terminal with given dimensions
// Returns handle or NULL on error
GomuxTerm gomux_term_new(unsigned int rows, unsigned int cols);

// Free terminal
void gomux_term_free(GomuxTerm term);

// Process bytes through VTE (parse escape sequences, update grid)
void gomux_term_process(GomuxTerm term, const char* data, size_t len);

// Get cell content at position
// Returns 0 on success, -1 if out of bounds
int gomux_term_get_cell(GomuxTerm term, unsigned int row, unsigned int col, 
                        char* out_char, size_t out_len);

// Get cursor position
// Returns: 0 on success, -1 on error
int gomux_term_get_cursor(GomuxTerm term, unsigned int* row, unsigned int* col);

// Resize terminal
void gomux_term_resize(GomuxTerm term, unsigned int rows, unsigned int cols);

// Get dimensions
unsigned int gomux_term_get_rows(GomuxTerm term);
unsigned int gomux_term_get_cols(GomuxTerm term);

// Legacy compatibility (now same as process)
#define gomux_pane_new gomux_term_new
#define gomux_pane_free gomux_term_free
#define gomux_pane_write gomux_term_process
#define gomux_pane_get_line gomux_term_get_line_wrapped
#define gomux_pane_get_cursor gomux_term_get_cursor
#define gomux_pane_tick gomux_term_noop
#define gomux_pane_needs_render gomux_term_needs_render
#define gomux_pane_mark_rendered gomux_term_mark_rendered

// For compatibility - get full line (for our current Go code)
int gomux_term_get_line_wrapped(GomuxTerm term, unsigned int row, char* buf, unsigned int max_len);
int gomux_term_needs_render(GomuxTerm term);
void gomux_term_mark_rendered(GomuxTerm term);
int gomux_term_noop(GomuxTerm term);

#ifdef __cplusplus
}
#endif

#endif // GOMUX_TERM_H
