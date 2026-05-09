package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestStatusBarView_ContainsIDLE proves zone 4 (system state) is always
// visible in the rendered output. Per ROADMAP Phase 1 success criterion #5:
// "View() renders a string containing the status bar".
func TestStatusBarView_ContainsIDLE(t *testing.T) {
	sb := NewStatusBar(80)
	out := sb.View().Content
	if !strings.Contains(out, "IDLE") {
		t.Fatalf("expected View() output to contain %q, got %q", "IDLE", out)
	}
}

// TestStatusBarView_ScanningShowsPills proves zones 2 and 3 only render
// when the state is SCANNING/EXECUTING.
func TestStatusBarView_ScanningShowsPills(t *testing.T) {
	sb := NewStatusBar(120)
	sb.State = "SCANNING"
	out := sb.View().Content
	if !strings.Contains(out, "VPX") {
		t.Fatalf("expected SCANNING View() to contain pill %q, got %q", "VPX", out)
	}
	if !strings.Contains(out, "10 / 30") {
		t.Fatalf("expected SCANNING View() to contain count label %q, got %q", "10 / 30", out)
	}
}

// TestStatusBarUpdate_WindowSizeMsg proves Update does not panic on resize
// (TUI-06 enforcement point at the component level).
func TestStatusBarUpdate_WindowSizeMsg(t *testing.T) {
	sb := NewStatusBar(80)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("StatusBar.Update panicked on WindowSizeMsg: %v", r)
		}
	}()
	_, _ = sb.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
}
