// Package catalog: match.go implements the fuzzy matching engine for the
// catalog package. It owns FindMatch (Path A same-era filter + Path B
// full-catalog WRatio fallback), BestMatch (threshold gating), and
// ForceMatch (exact MasterID / IPDBNum lookup). The *Catalog receiver
// methods declared here are the sole implementations; catalog.go contains
// only construction + Load() logic.
package catalog

import (
	"sort"
	"strings"

	fuzzy "github.com/paul-mannino/go-fuzzywuzzy"

	"github.com/iainconnor/vpin-plunger/internal/naming"
)

// findMatchPathA scores entries that pass the same-era filter (manufacturer
// case-insensitive match + |catalog.Year - signal.Year| <= cfg.YearWindow).
// D-04: ±window, NOT exact-year as the original migration brief specified.
func (c *Catalog) findMatchPathA(sig *naming.Signal) []MatchResult {
	var out []MatchResult
	qNorm := strings.ToLower(naming.Normalize(sig.Name, c.cfg.TrailingArticle))
	for _, e := range c.entries {
		if !strings.EqualFold(strings.TrimSpace(e.Manufacturer), strings.TrimSpace(sig.Manufacturer)) {
			continue
		}
		diff := e.Year - sig.Year
		if diff < 0 {
			diff = -diff
		}
		if diff > c.cfg.YearWindow {
			continue
		}
		cNorm := strings.ToLower(naming.Normalize(e.Name, c.cfg.TrailingArticle))
		score := fuzzy.WRatio(qNorm, cNorm)
		out = append(out, MatchResult{Entry: e, Confidence: score, MatchField: "game_name"})
	}
	sortByConfidence(out)
	return out
}

// findMatchPathB scores every entry against the noise-stripped query stem.
// Used when no signal was extracted, or when Path A's best score is below
// the interactive threshold (D-03 fallback).
func (c *Catalog) findMatchPathB(stem string) []MatchResult {
	qNorm := strings.ToLower(naming.NormalizeForMatching(stem, c.cfg.TrailingArticle))
	out := make([]MatchResult, 0, len(c.entries))
	for _, e := range c.entries {
		cNorm := strings.ToLower(naming.NormalizeForMatching(e.Name, c.cfg.TrailingArticle))
		score := fuzzy.WRatio(qNorm, cNorm)
		out = append(out, MatchResult{Entry: e, Confidence: score, MatchField: "game_name"})
	}
	sortByConfidence(out)
	return out
}

// sortByConfidence sorts a MatchResult slice in descending confidence order.
func sortByConfidence(rs []MatchResult) {
	sort.SliceStable(rs, func(i, j int) bool {
		return rs[i].Confidence > rs[j].Confidence
	})
}

// topN returns the first limit elements of rs, or all of rs when limit <= 0
// or limit >= len(rs).
func topN(rs []MatchResult, limit int) []MatchResult {
	if limit <= 0 || limit >= len(rs) {
		return rs
	}
	return rs[:limit]
}

// FindMatch implements CAT-05: Path A same-era filter with Path B full-catalog
// WRatio fallback. Returns up to limit candidates ordered by descending
// confidence. Returns ErrNotLoaded when Load() has not yet been called.
func (c *Catalog) FindMatch(stem string, limit int) ([]MatchResult, error) {
	if c.entries == nil {
		return nil, ErrNotLoaded
	}
	sig := naming.ExtractSignal(stem)
	if sig != nil {
		pathA := c.findMatchPathA(sig)
		if len(pathA) > 0 && pathA[0].Confidence >= ThresholdInteractive {
			return topN(pathA, limit), nil
		}
	}
	return topN(c.findMatchPathB(stem), limit), nil
}

// BestMatch implements CAT-06: returns the top FindMatch candidate when its
// confidence is at least ThresholdInteractive (72). Returns nil otherwise.
// Returns ErrNotLoaded when Load() has not yet been called.
// Auto-assignable when result.Confidence >= ThresholdAutoAssign (92).
func (c *Catalog) BestMatch(stem string) (*MatchResult, error) {
	results, err := c.FindMatch(stem, 1)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	if results[0].Confidence < ThresholdInteractive {
		return nil, nil
	}
	r := results[0]
	return &r, nil
}

// ForceMatch implements CAT-07: exact (case-insensitive, whitespace-trimmed)
// match against MasterID first, then IPDBNum. Returns Confidence=100 with
// MatchField identifying which column matched, or nil when neither matches.
// Returns ErrNotLoaded when Load() has not yet been called.
func (c *Catalog) ForceMatch(id string) (*MatchResult, error) {
	if c.entries == nil {
		return nil, ErrNotLoaded
	}
	idTrim := strings.TrimSpace(id)
	if idTrim == "" {
		return nil, nil
	}
	for _, e := range c.entries {
		if strings.EqualFold(strings.TrimSpace(e.MasterID), idTrim) {
			r := MatchResult{Entry: e, Confidence: 100, MatchField: "master_id"}
			return &r, nil
		}
	}
	for _, e := range c.entries {
		if strings.EqualFold(strings.TrimSpace(e.IPDBNum), idTrim) {
			r := MatchResult{Entry: e, Confidence: 100, MatchField: "ipdb_num"}
			return &r, nil
		}
	}
	return nil, nil
}
