package planner

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// FormatPlan produces a human-readable dry-run report for a ProcessPlan.
// The output is a plain string with no ANSI escape codes. The app layer
// (Phase 6) optionally wraps the output with lipgloss styling.
//
// Format:
//   - Top-level actions printed with their action label, source/virtual path, and destination.
//   - EXTRACT_ARCHIVE children indented 2 spaces.
//   - SKIP actions shown in brackets: [SKIP] ...
//   - Summary table at the bottom listing non-zero action type counts.
//
// PLN-06: tree-format plan with action labels, virtual paths, and summary counts.
// D-06: pure function returning a plain string; no prints, no I/O.
func FormatPlan(plan *ProcessPlan) string {
	var sb strings.Builder

	// Column width for action label left-padding
	const labelWidth = 16

	counts := make(map[ActionType]int)

	for _, action := range plan.Actions {
		formatAction(&sb, action, 0, labelWidth, counts)
	}

	// Summary table
	sb.WriteString("\nSummary:\n")
	// Emit types in a stable display order
	order := []ActionType{
		ActionTypeExtractArchive,
		ActionTypeVaultArchive,
		ActionTypeMoveVPX,
		ActionTypeMoveBackglass,
		ActionTypeMoveROM,
		ActionTypeMoveNVRAM,
		ActionTypeMovePOV,
		ActionTypeMoveUltraDMD,
		ActionTypeMoveFlexDMD,
		ActionTypeMoveAudio,
		ActionTypeMoveAltcolor,
		ActionTypeMoveMusic,
		ActionTypeMovePUP,
		ActionTypeRegisterGame,
		ActionTypeSendToReview,
		ActionTypeIgnore,
		ActionTypeSkip,
	}
	// Also include any types not in the order list (future-proofing)
	inOrder := make(map[ActionType]bool)
	for _, t := range order {
		inOrder[t] = true
	}
	var extra []ActionType
	for t := range counts {
		if !inOrder[t] {
			extra = append(extra, t)
		}
	}
	sort.Slice(extra, func(i, j int) bool { return int(extra[i]) < int(extra[j]) })
	order = append(order, extra...)

	for _, t := range order {
		if n := counts[t]; n > 0 {
			fmt.Fprintf(&sb, "  %-20s %d\n", t.String(), n)
		}
	}

	return sb.String()
}

// formatAction writes one action (and its children) to sb with the given indent level.
func formatAction(sb *strings.Builder, action *PlannedAction, indent int, labelWidth int, counts map[ActionType]int) {
	counts[action.Type]++

	prefix := strings.Repeat(" ", indent)

	// Build display name: prefer VirtualPath for archive members, else Source basename
	displaySource := action.Source
	if action.VirtualPath != "" {
		displaySource = action.VirtualPath
	} else if displaySource != "" {
		displaySource = filepath.Base(displaySource)
	}

	label := action.Type.String()
	if action.Type == ActionTypeSkip {
		label = "[SKIP]"
	}

	// Pad label to labelWidth for alignment (only at indent=0; children use natural width)
	paddedLabel := label
	if indent == 0 && len(label) < labelWidth {
		paddedLabel = label + strings.Repeat(" ", labelWidth-len(label))
	}

	line := prefix + paddedLabel
	if displaySource != "" {
		line += "  " + displaySource
	}
	if action.Dest != "" && action.Type != ActionTypeExtractArchive {
		line += "  →  " + action.Dest
	}
	if action.Type == ActionTypeSkip && action.SupersededBy != "" {
		line += "  (superseded by " + filepath.Base(action.SupersededBy) + ")"
	}
	if action.Type == ActionTypeSendToReview && action.Reason != "" {
		line += "  (" + action.Reason + ")"
	}
	sb.WriteString(line + "\n")

	// Recurse into children (EXTRACT_ARCHIVE)
	for _, child := range action.Children {
		formatAction(sb, child, indent+2, labelWidth, counts)
	}
}
