package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"shux/internal/persist"
	"shux/internal/protocol"
)

// Ls lists on-disk resurrection store (manifest + journals).
func Ls(ctx context.Context, stateDir string, jsonOutput bool) error {
	sum, err := persist.InspectState(stateDir)
	if err != nil {
		return fmt.Errorf("client: ls: %w", err)
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(sum)
	}
	writeLsSummary(os.Stdout, sum)
	return nil
}

// Prune removes orphan journal files not referenced by the manifest.
func Prune(ctx context.Context, stateDir string, dryRun, jsonOutput bool) error {
	m, ok, err := persist.LoadManifest(stateDir)
	if err != nil {
		return fmt.Errorf("client: prune: load manifest: %w", err)
	}
	if !ok {
		m = persist.Manifest{}
	}
	if dryRun {
		sum, err := persist.InspectState(stateDir)
		if err != nil {
			return fmt.Errorf("client: prune: %w", err)
		}
		var orphans []string
		for _, j := range sum.Journals {
			if !j.Referenced {
				orphans = append(orphans, j.Path)
			}
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{
				"dry_run": true,
				"pruned":  orphans,
			})
		}
		if len(orphans) == 0 {
			fmt.Println("nothing to prune")
			return nil
		}
		for _, path := range orphans {
			fmt.Println(path)
		}
		return nil
	}
	removed, err := persist.PruneOrphanJournals(stateDir, m)
	if err != nil {
		return fmt.Errorf("client: prune: %w", err)
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"pruned": removed})
	}
	if len(removed) == 0 {
		fmt.Println("nothing to prune")
		return nil
	}
	for _, path := range removed {
		fmt.Println(path)
	}
	return nil
}

// Rm removes the manifest and all pane journals from the store.
func Rm(ctx context.Context, addr, stateDir string, force, jsonOutput bool) error {
	available, err := ServerAvailable(ctx, addr)
	if err == nil && available && !force {
		return fmt.Errorf("client: rm: daemon is running; stop it first or pass --force")
	}
	if err := persist.ClearResurrectionState(stateDir); err != nil {
		return fmt.Errorf("client: rm: %w", err)
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"removed": stateDir})
	}
	fmt.Printf("removed store %s\n", stateDir)
	return nil
}

// Checkpoint asks the running daemon to save a resurrection checkpoint.
func Checkpoint(ctx context.Context, addr string, jsonOutput bool) error {
	available, err := ServerAvailable(ctx, addr)
	if err != nil {
		return err
	}
	if !available {
		return fmt.Errorf("client: checkpoint: no daemon listening on %s", addr)
	}
	resp, err := runQuery(ctx, addr, protocol.QueryRequest{Method: protocol.QueryCheckpointState})
	if err != nil {
		return fmt.Errorf("client: checkpoint: %w", err)
	}
	if resp.Checkpoint == nil {
		return fmt.Errorf("client: checkpoint: daemon returned empty response")
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(resp.Checkpoint)
	}
	if len(resp.Checkpoint.Pruned) == 0 {
		fmt.Println("checkpoint saved")
		return nil
	}
	fmt.Println("checkpoint saved; pruned orphan journals:")
	for _, path := range resp.Checkpoint.Pruned {
		fmt.Println(path)
	}
	return nil
}

func writeLsSummary(out io.Writer, sum persist.StateSummary) {
	fmt.Fprintf(out, "STORE\t%s\n", sum.StateDir)
	if !sum.ManifestExists {
		fmt.Fprintln(out, "MANIFEST\t(none)")
	} else {
		m := sum.Manifest
		tier := m.RecoveryTier
		if tier == "" {
			tier = "l2"
		}
		fmt.Fprintf(out, "MANIFEST\tv%d tier=%s sessions=%d default=%s\n",
			m.Version, tier, len(m.Sessions), m.DefaultSessionName)
	}
	if len(sum.Journals) == 0 {
		fmt.Fprintln(out, "\nno journals")
		return
	}
	fmt.Fprintln(out)
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "JOURNAL\tSIZE\tIN MANIFEST")
	for _, j := range sum.Journals {
		ref := "no"
		if j.Referenced {
			ref = "yes"
		}
		name := filepath.Base(j.Path)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", name, formatBytes(j.Size), ref)
	}
	_ = w.Flush()
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
