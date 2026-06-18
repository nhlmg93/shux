package protocol

import "testing"

func TestValidateEvent_windowLayoutChanged_zoomMetadata(t *testing.T) {
	valid := EventWindowLayoutChanged{
		SessionID:    "s-1",
		WindowID:     "w-1",
		Revision:     3,
		Cols:         80,
		Rows:         24,
		Panes:        []EventLayoutPane{{PaneID: "p-2", Col: 0, Row: 0, Cols: 80, Rows: 24}},
		ZoomedPaneID: "p-2",
		SavedPanes: []EventLayoutPane{
			{PaneID: "p-1", Col: 0, Row: 0, Cols: 40, Rows: 24},
			{PaneID: "p-2", Col: 40, Row: 0, Cols: 40, Rows: 24},
		},
	}
	if err := ValidateEvent(valid); err != nil {
		t.Fatalf("ValidateEvent(valid layout zoom metadata) error = %v", err)
	}

	invalidSaved := valid
	invalidSaved.SavedPanes = []EventLayoutPane{{PaneID: "", Col: 0, Row: 0, Cols: 40, Rows: 24}}
	if err := ValidateEvent(invalidSaved); err == nil {
		t.Fatal("ValidateEvent(saved pane invalid id) expected error")
	}

	invalidSaved = valid
	invalidSaved.SavedPanes = []EventLayoutPane{{PaneID: "p-1", Col: 0, Row: 0, Cols: 0, Rows: 24}}
	if err := ValidateEvent(invalidSaved); err == nil {
		t.Fatal("ValidateEvent(saved pane invalid size) expected error")
	}
}
