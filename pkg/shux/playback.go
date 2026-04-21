package shux

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PaneVisualSnapshot captures the visual state of a pane for recovery.
// This is separate from the structural snapshot and provides better UX
// when restoring after owner death or reboot.
type PaneVisualSnapshot struct {
	Version       int
	PaneID        uint32
	Title         string
	CursorRow     int
	CursorCol     int
	CursorVisible bool
	InAltScreen   bool
	BellCount     uint64
	Rows          int
	Cols          int
	Lines         []string     // Last rendered screen content (text only)
	Cells         [][]PaneCell // Full cell data if available
	VTLog         []byte       // Optional bounded VT byte ring for replay
	UpdatedAtUnix int64
}

// VisualSnapshotConfig configures visual snapshot behavior.
type VisualSnapshotConfig struct {
	// MaxLines is the maximum scrollback lines to save (0 = disabled)
	MaxLines int

	// MaxVTLogBytes is the maximum VT byte log to save (0 = disabled)
	MaxVTLogBytes int

	// SaveInterval is how often to save visual state
	SaveInterval time.Duration
}

// DefaultVisualSnapshotConfig returns sensible defaults.
func DefaultVisualSnapshotConfig() VisualSnapshotConfig {
	return VisualSnapshotConfig{
		MaxLines:      1000,
		MaxVTLogBytes: 64 * 1024, // 64KB
		SaveInterval:  5 * time.Second,
	}
}

// PlaybackManager manages visual snapshots and VT logs for all panes.
type PlaybackManager struct {
	config VisualSnapshotConfig
	logger ShuxLogger

	// Storage paths
	baseDir string
}

// NewPlaybackManager creates a new playback manager for a session.
func NewPlaybackManager(sessionName string, config VisualSnapshotConfig, logger ShuxLogger) (*PlaybackManager, error) {
	baseDir := filepath.Join(SessionDir(sessionName), "panes")
	if err := os.MkdirAll(baseDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create playback directory: %w", err)
	}

	return &PlaybackManager{
		config:  config,
		logger:  logger,
		baseDir: baseDir,
	}, nil
}

// SaveVisualSnapshot saves the visual state of a pane.
func (pm *PlaybackManager) SaveVisualSnapshot(runtime *PaneRuntime) error {
	if runtime == nil {
		return fmt.Errorf("cannot save nil runtime")
	}

	// Build content from runtime
	content := runtime.BuildContent()

	snapshot := PaneVisualSnapshot{
		Version:       1,
		PaneID:        runtime.ID(),
		Title:         content.Title,
		CursorRow:     content.CursorRow,
		CursorCol:     content.CursorCol,
		CursorVisible: !content.CursorHidden,
		InAltScreen:   content.InAltScreen,
		BellCount:     content.BellCount,
		Rows:          len(content.Cells),
		Cols:          0,
		Lines:         content.Lines,
		Cells:         content.Cells,
		UpdatedAtUnix: time.Now().Unix(),
	}

	if len(content.Cells) > 0 {
		snapshot.Cols = len(content.Cells[0])
	}

	// Trim cells if needed (save memory)
	if pm.config.MaxLines > 0 && len(snapshot.Cells) > pm.config.MaxLines {
		snapshot.Cells = snapshot.Cells[:pm.config.MaxLines]
	}

	path := pm.snapshotPath(runtime.ID())
	if err := pm.writeSnapshot(path, &snapshot); err != nil {
		return fmt.Errorf("failed to write visual snapshot: %w", err)
	}

	if pm.logger != nil {
		pm.logger.Infof("playback: saved visual snapshot pane=%d rows=%d cols=%d", snapshot.PaneID, snapshot.Rows, snapshot.Cols)
	}

	return nil
}

// LoadVisualSnapshot loads the visual snapshot for a pane.
func (pm *PlaybackManager) LoadVisualSnapshot(paneID uint32) (*PaneVisualSnapshot, error) {
	path := pm.snapshotPath(paneID)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No snapshot exists
		}
		return nil, fmt.Errorf("failed to read visual snapshot: %w", err)
	}

	snapshot, err := pm.decodeSnapshot(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode visual snapshot: %w", err)
	}

	return snapshot, nil
}

// DeleteVisualSnapshot removes a pane's visual snapshot.
func (pm *PlaybackManager) DeleteVisualSnapshot(paneID uint32) error {
	path := pm.snapshotPath(paneID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete visual snapshot: %w", err)
	}
	return nil
}

// ListVisualSnapshots returns all available visual snapshots.
func (pm *PlaybackManager) ListVisualSnapshots() ([]uint32, error) {
	entries, err := os.ReadDir(pm.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	var paneIDs []uint32
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Parse pane ID from filename (format: <id>.screen.gob)
		var paneID uint32
		if _, err := fmt.Sscanf(name, "%d.screen.gob", &paneID); err == nil {
			paneIDs = append(paneIDs, paneID)
		}
	}

	return paneIDs, nil
}

// CleanupStaleSnapshots removes visual snapshots for panes that no longer exist.
func (pm *PlaybackManager) CleanupStaleSnapshots(validPaneIDs []uint32) error {
	validSet := make(map[uint32]bool)
	for _, id := range validPaneIDs {
		validSet[id] = true
	}

	existing, err := pm.ListVisualSnapshots()
	if err != nil {
		return err
	}

	for _, paneID := range existing {
		if !validSet[paneID] {
			if err := pm.DeleteVisualSnapshot(paneID); err != nil {
				if pm.logger != nil {
					pm.logger.Warnf("playback: failed to cleanup stale snapshot pane=%d: %v", paneID, err)
				}
			}
		}
	}

	return nil
}

// snapshotPath returns the file path for a pane's visual snapshot.
func (pm *PlaybackManager) snapshotPath(paneID uint32) string {
	return filepath.Join(pm.baseDir, fmt.Sprintf("%d.screen.gob", paneID))
}

// writeSnapshot atomically writes a snapshot to disk.
func (pm *PlaybackManager) writeSnapshot(path string, snapshot *PaneVisualSnapshot) error {
	tmpPath := path + ".tmp"

	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Use gob encoding for now (can be optimized later)
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(snapshot); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, path)
}

// decodeSnapshot decodes a snapshot from bytes using gob.
func (pm *PlaybackManager) decodeSnapshot(data []byte) (*PaneVisualSnapshot, error) {
	var snapshot PaneVisualSnapshot
	decoder := gob.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("gob decode failed: %w", err)
	}
	return &snapshot, nil
}

// VisualRecoveryResult contains the result of a visual recovery attempt.
type VisualRecoveryResult struct {
	PaneID         uint32
	Success        bool
	Source         string // "live", "visual_snapshot", "blank"
	Content        *PaneContent
	VisualSnapshot *PaneVisualSnapshot
}

// RecoverVisualState attempts to recover the visual state for a pane.
// Preference order:
// 1. Live runtime if available
// 2. Visual snapshot if available
// 3. Blank shell as last resort
func (pm *PlaybackManager) RecoverVisualState(paneID uint32, runtime *PaneRuntime) *VisualRecoveryResult {
	// First choice: live runtime
	if runtime != nil && !runtime.IsClosed() {
		content := runtime.BuildContent()
		return &VisualRecoveryResult{
			PaneID:  paneID,
			Success: true,
			Source:  "live",
			Content: content,
		}
	}

	// Second choice: visual snapshot
	visualSnapshot, err := pm.LoadVisualSnapshot(paneID)
	if err == nil && visualSnapshot != nil {
		content := &PaneContent{
			Lines:        visualSnapshot.Lines,
			Cells:        visualSnapshot.Cells,
			CursorRow:    visualSnapshot.CursorRow,
			CursorCol:    visualSnapshot.CursorCol,
			InAltScreen:  visualSnapshot.InAltScreen,
			CursorHidden: !visualSnapshot.CursorVisible,
			Title:        visualSnapshot.Title,
			BellCount:    visualSnapshot.BellCount,
		}
		return &VisualRecoveryResult{
			PaneID:         paneID,
			Success:        true,
			Source:         "visual_snapshot",
			Content:        content,
			VisualSnapshot: visualSnapshot,
		}
	}

	// Last resort: blank
	return &VisualRecoveryResult{
		PaneID:  paneID,
		Success: false,
		Source:  "blank",
		Content: &PaneContent{
			Lines: make([]string, 0),
			Cells: make([][]PaneCell, 0),
		},
	}
}

// VTLogBuffer is a bounded ring buffer for VT bytes.
// This allows replay of terminal output for better recovery.
type VTLogBuffer struct {
	maxSize int
	data    []byte
	size    int
	start   int
}

// NewVTLogBuffer creates a new VT log buffer.
func NewVTLogBuffer(maxSize int) *VTLogBuffer {
	return &VTLogBuffer{
		maxSize: maxSize,
		data:    make([]byte, maxSize),
	}
}

// Write writes data to the buffer.
func (b *VTLogBuffer) Write(data []byte) {
	if b.maxSize == 0 {
		return
	}

	for _, byte := range data {
		b.data[(b.start+b.size)%b.maxSize] = byte
		if b.size < b.maxSize {
			b.size++
		} else {
			b.start = (b.start + 1) % b.maxSize
		}
	}
}

// Read reads the buffered data.
func (b *VTLogBuffer) Read() []byte {
	if b.size == 0 {
		return nil
	}

	result := make([]byte, b.size)
	for i := 0; i < b.size; i++ {
		result[i] = b.data[(b.start+i)%b.maxSize]
	}
	return result
}

// Clear clears the buffer.
func (b *VTLogBuffer) Clear() {
	b.size = 0
	b.start = 0
}

// Size returns the current size of the buffer.
func (b *VTLogBuffer) Size() int {
	return b.size
}
