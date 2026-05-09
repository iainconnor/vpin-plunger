package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestModelView_ContainsIDLE is the canonical Phase 1 smoke test
// (ROADMAP Phase 1 success criterion #5):
// "View() renders a string containing the status bar".
func TestModelView_ContainsIDLE(t *testing.T) {
	m := New()
	out := m.View().Content
	if !strings.Contains(out, "IDLE") {
		t.Fatalf("expected Model.View() to contain %q, got %q", "IDLE", out)
	}
}

// TestModelUpdate_WindowSizeMsgNoPanic is the second canonical Phase 1
// smoke test: "WindowSizeMsg is handled without panic". Also verifies the
// Width is propagated to the embedded StatusBar (TUI-06).
func TestModelUpdate_WindowSizeMsgNoPanic(t *testing.T) {
	m := New()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Model.Update panicked on WindowSizeMsg: %v", r)
		}
	}()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 132, Height: 40})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update did not return Model, got %T", updated)
	}
	if got.Width != 132 {
		t.Fatalf("expected Width to propagate to 132, got %d", got.Width)
	}
	if got.Height != 40 {
		t.Fatalf("expected Height to propagate to 40, got %d", got.Height)
	}
	if got.StatusBar.Width != 132 {
		t.Fatalf("expected StatusBar.Width to propagate to 132, got %d", got.StatusBar.Width)
	}
}

// TestModelInit_ReturnsCmd guards bubbletea v2 signature compliance.
// In bubbletea v2, Init() returns tea.Cmd (not (tea.Model, tea.Cmd) as in v1).
func TestModelInit_ReturnsCmd(t *testing.T) {
	m := New()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Model.Init() panicked: %v", r)
		}
	}()
	_ = m.Init()
}
