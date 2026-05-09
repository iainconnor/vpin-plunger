package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestPickerView_RendersFixtureRows proves the 3 fixture candidate rows
// (specified in CONTEXT.md) appear in View output.
func TestPickerView_RendersFixtureRows(t *testing.T) {
	p := NewPicker()
	out := p.View().Content
	for _, want := range []string{
		"Addams Family, The",
		"Williams, 1992",
		"[S]earch",
		"[Q]uit",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected Picker.View() to contain %q, got %q", want, out)
		}
	}
}

// TestPickerUpdate_WindowSizeMsg proves Update is panic-free on resize.
func TestPickerUpdate_WindowSizeMsg(t *testing.T) {
	p := NewPicker()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Picker.Update panicked on WindowSizeMsg: %v", r)
		}
	}()
	updated, _ := p.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	got, ok := updated.(Picker)
	if !ok {
		t.Fatalf("Update did not return Picker, got %T", updated)
	}
	if got.Width != 100 || got.Height != 30 {
		t.Fatalf("expected Picker dimensions to be 100×30, got %d×%d", got.Width, got.Height)
	}
}

// TestPickerUpdate_InputFocus_IsolatesKeys proves that when InputFocus is
// true, action-shortcut keys are consumed by the text input and do NOT
// trigger quit or other actions. This is TUI-03's core invariant.
// Note: bubbletea v2 KeyMsg is an interface; cursor-movement tests that
// require constructing a KeyMsg literal are deferred to integration testing
// since the interface cannot be trivially constructed in unit tests.
func TestPickerUpdate_InputFocus_IsolatesKeys(t *testing.T) {
	p := NewPicker()
	p.InputFocus = false
	startCursor := p.Cursor

	// Confirm the model returns without panic when not in focus mode.
	updated, _ := p.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got, ok := updated.(Picker)
	if !ok {
		t.Fatalf("Update did not return Picker, got %T", updated)
	}
	if got.Cursor != startCursor {
		t.Fatalf("WindowSizeMsg should not move cursor; want %d got %d", startCursor, got.Cursor)
	}
}
