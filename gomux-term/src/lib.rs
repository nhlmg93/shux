//! gomux-term: Alacritty-based terminal emulator for Go
//! 
//! Replaces PaneActor/Grid with TermActor that uses Alacritty's
//! terminal emulation (vte parser, Grid, colors, etc.)

pub mod term;

// Re-export FFI functions at crate root for C ABI
pub use term::*;
