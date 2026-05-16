package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/iainconnor/vpin-plunger/internal/catalog"
	"github.com/iainconnor/vpin-plunger/internal/planner"
)

// Helper: run Update once and return the resulting Model (panics if cast fails).
func step(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	nm, ok := next.(Model)
	if !ok {
		t.Fatalf("Update did not return Model, got %T", next)
	}
	return nm
}

func TestMatchFnChannelPair(t *testing.T) {
	m := New()
	respCh := make(chan planner.MatchChoice, 1)
	req := MatchRequestMsg(MatchRequest{
		Stem: "test stem",
		Candidates: []catalog.MatchResult{
			{Entry: catalog.SheetEntry{Name: "Addams Family", Manufacturer: "Williams", Year: 1992}, Confidence: 95},
			{Entry: catalog.SheetEntry{Name: "Addams Family Values", Manufacturer: "Williams", Year: 1993}, Confidence: 70},
		},
		Response: respCh,
	})
	m = step(t, m, req)
	if m.PendingMatch == nil {
		t.Fatal("PendingMatch should be set after MatchRequestMsg")
	}
	if m.State != StateMatching {
		t.Errorf("State = %q, want %q", m.State, StateMatching)
	}
	if len(m.Picker.Candidates) != 2 {
		t.Errorf("Picker should have 2 candidates, got %d", len(m.Picker.Candidates))
	}

	// Simulate enter press with cursor on first candidate.
	m.Picker.Cursor = 0
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	select {
	case choice := <-respCh:
		if choice.Match == nil {
			t.Fatal("expected MatchChoice.Match to be set")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for MatchChoice on Response channel")
	}
	if m.PendingMatch != nil {
		t.Error("PendingMatch should be nil after enter")
	}
}

func TestConfirmPrompt(t *testing.T) {
	m := New()
	m.AutoMode = false
	fakePlan := &planner.ProcessPlan{}
	m = step(t, m, ScanDoneMsg{Plan: fakePlan})
	if m.State != StateConfirming {
		t.Errorf("State = %q, want %q", m.State, StateConfirming)
	}
	if !m.Confirming {
		t.Error("Confirming should be true")
	}
	if len(m.Picker.Candidates) != 2 {
		t.Errorf("Picker should have 2 candidates, got %d", len(m.Picker.Candidates))
	}
	if m.Picker.Cursor != 0 {
		t.Errorf("Picker.Cursor = %d, want 0", m.Picker.Cursor)
	}
	if !strings.Contains(strings.ToLower(m.Picker.Candidates[0].Name), "execute") {
		t.Errorf("first candidate = %q, want one containing 'execute'", m.Picker.Candidates[0].Name)
	}
}

func TestProgressMsg(t *testing.T) {
	m := New()
	m = step(t, m, ExecuteProgressMsg{Moved: 3, Failed: 1, Reviewed: 2, Ignored: 0})
	if m.ProgressCounts.Moved != 3 || m.ProgressCounts.Failed != 1 || m.ProgressCounts.Reviewed != 2 {
		t.Errorf("ProgressCounts = %+v, want {3,1,2,0}", m.ProgressCounts)
	}
	s := m.StatusBar.State
	for _, want := range []string{"3", "1", "2"} {
		if !strings.Contains(s, want) {
			t.Errorf("StatusBar.State = %q missing %q", s, want)
		}
	}
}

func TestAutoMode(t *testing.T) {
	m := New()
	m.AutoMode = true
	fakePlan := &planner.ProcessPlan{}
	m = step(t, m, ScanDoneMsg{Plan: fakePlan})
	if m.State != StateExecuting {
		t.Errorf("State = %q, want %q (auto skips confirm)", m.State, StateExecuting)
	}
	if m.Confirming {
		t.Error("Confirming must remain false in --auto")
	}
}
