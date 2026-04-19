#ifndef GOMUX_TERM_H
#define GOMUX_TERM_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Opaque handle to terminal instance
typedef void* GomuxTerm;

// Create a new terminal with PTY
// Returns handle or NULL on error
GomuxTerm gomux_term_new(unsigned int rows, unsigned int cols);

// Free a terminal instance
void gomux_term_free(GomuxTerm term);

// Process bytes from PTY (read PTY and update grid)
// Returns: bytes read (0=EOF, -1=error, >0=bytes processed)
int gomux_term_process_pty(GomuxTerm term);

// Write bytes to PTY (send user input)
// Returns: 0 on success, -1 on error
int gomux_term_write(GomuxTerm term, const char* data, unsigned int len);

// Get a line from the grid for rendering
// Copies up to max_len chars into buf, returns actual length
// buf must be at least max_len bytes
int gomux_term_get_line(GomuxTerm term, unsigned int row, char* buf, unsigned int max_len);

// Get cursor position
// Returns: 0 on success, -1 on error
int gomux_term_get_cursor(GomuxTerm term, unsigned int* row, unsigned int* col);

// Check if PTY has data available (for polling)
// Returns: 1 if data available, 0 if timeout, -1 on error
int gomux_term_poll_pty(GomuxTerm term, int timeout_ms);

#ifdef __cplusplus
}
#endif

#endif // GOMUX_TERM_H
