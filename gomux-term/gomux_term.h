#ifndef GOMUX_TERM_H
#define GOMUX_TERM_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Opaque handle to TermActor (replaces PaneActor)
typedef void* GomuxPane;

// Create new TermActor with PTY and shell
// Returns handle or NULL on error
GomuxPane gomux_pane_new(unsigned int rows, unsigned int cols, const char* shell);

// Free TermActor (kills PTY process)
void gomux_pane_free(GomuxPane pane);

// Process PTY output (read from shell, update grid)
// Returns: 1 if content changed, 0 if no data, -1 if error
int gomux_pane_tick(GomuxPane pane);

// Write input to PTY (send keys to shell)
// Returns: 0 on success, -1 on error
int gomux_pane_write(GomuxPane pane, const char* data, unsigned int len);

// Get rendered line from grid
// Copies up to max_len chars into buf, returns actual length
int gomux_pane_get_line(GomuxPane pane, unsigned int row, char* buf, unsigned int max_len);

// Get cursor position
// Returns: 0 on success, -1 on error
int gomux_pane_get_cursor(GomuxPane pane, unsigned int* row, unsigned int* col);

// Check if grid changed since last mark_rendered
// Returns: 1 if dirty, 0 if clean, -1 if error
int gomux_pane_needs_render(GomuxPane pane);

// Mark as rendered (reset dirty flag)
void gomux_pane_mark_rendered(GomuxPane pane);

// Legacy compatibility - same as tick
int gomux_term_process_pty(void* term);
int gomux_term_poll_pty(void* term, int timeout_ms);

#ifdef __cplusplus
}
#endif

#endif // GOMUX_TERM_H
