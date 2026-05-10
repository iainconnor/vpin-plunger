// Package naming provides the filename normalization pipeline and canonical
// name construction used by the catalog package. All functions are pure,
// stateless, and safe for concurrent use. This package has zero external
// dependencies and is a leaf in the import graph: it must NOT import
// internal/formats, internal/catalog, internal/ui, or internal/app.
package naming

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Signal is the structured form extracted from a VPX filename stem.
type Signal struct {
	Name         string
	Manufacturer string
	Year         int
}

var (
	// Possessive strip: matches both straight and curly apostrophe + s + word boundary.
	rePossessive = regexp.MustCompile(`['']s\b`)

	// CamelCase splitting (two passes):
	// - lower-then-Upper: "GrandPrix" -> "Grand Prix"
	// - Upper-then-Upper-then-lower: "VPXGame" -> "VPX Game" (acronym boundary)
	reCamelLower = regexp.MustCompile(`([a-z])([A-Z])`)
	reCamelUpper = regexp.MustCompile(`([A-Z])([A-Z][a-z])`)

	// Roman numerals II..XX (I excluded). MUST be ordered longest-first so that
	// XX is replaced before XIX, XIX before XVIII, etc., to prevent partial
	// replacement (e.g. X inside XI). \b word boundaries on both sides.
	romanNumerals = []struct {
		pat *regexp.Regexp
		val string
	}{
		{regexp.MustCompile(`\bXX\b`), "20"},
		{regexp.MustCompile(`\bXIX\b`), "19"},
		{regexp.MustCompile(`\bXVIII\b`), "18"},
		{regexp.MustCompile(`\bXVII\b`), "17"},
		{regexp.MustCompile(`\bXVI\b`), "16"},
		{regexp.MustCompile(`\bXV\b`), "15"},
		{regexp.MustCompile(`\bXIV\b`), "14"},
		{regexp.MustCompile(`\bXIII\b`), "13"},
		{regexp.MustCompile(`\bXII\b`), "12"},
		{regexp.MustCompile(`\bXI\b`), "11"},
		{regexp.MustCompile(`\bX\b`), "10"},
		{regexp.MustCompile(`\bIX\b`), "9"},
		{regexp.MustCompile(`\bVIII\b`), "8"},
		{regexp.MustCompile(`\bVII\b`), "7"},
		{regexp.MustCompile(`\bVI\b`), "6"},
		{regexp.MustCompile(`\bV\b`), "5"},
		{regexp.MustCompile(`\bIV\b`), "4"},
		{regexp.MustCompile(`\bIII\b`), "3"},
		{regexp.MustCompile(`\bII\b`), "2"},
		// I intentionally excluded — too ambiguous with the pronoun
	}

	// Number words 0..12 — cardinals and ordinals. Longest-first.
	// Case-insensitive flag (?i). \b boundaries.
	numberWords = []struct {
		pat *regexp.Regexp
		val string
	}{
		{regexp.MustCompile(`(?i)\btwelfth\b`), "12"},
		{regexp.MustCompile(`(?i)\btwelve\b`), "12"},
		{regexp.MustCompile(`(?i)\beleventh\b`), "11"},
		{regexp.MustCompile(`(?i)\beleven\b`), "11"},
		{regexp.MustCompile(`(?i)\btenth\b`), "10"},
		{regexp.MustCompile(`(?i)\bten\b`), "10"},
		{regexp.MustCompile(`(?i)\bninth\b`), "9"},
		{regexp.MustCompile(`(?i)\bnine\b`), "9"},
		{regexp.MustCompile(`(?i)\beighth\b`), "8"},
		{regexp.MustCompile(`(?i)\beight\b`), "8"},
		{regexp.MustCompile(`(?i)\bseventh\b`), "7"},
		{regexp.MustCompile(`(?i)\bseven\b`), "7"},
		{regexp.MustCompile(`(?i)\bsixth\b`), "6"},
		{regexp.MustCompile(`(?i)\bsix\b`), "6"},
		{regexp.MustCompile(`(?i)\bfifth\b`), "5"},
		{regexp.MustCompile(`(?i)\bfive\b`), "5"},
		{regexp.MustCompile(`(?i)\bfourth\b`), "4"},
		{regexp.MustCompile(`(?i)\bfour\b`), "4"},
		{regexp.MustCompile(`(?i)\bthird\b`), "3"},
		{regexp.MustCompile(`(?i)\bthree\b`), "3"},
		{regexp.MustCompile(`(?i)\bsecond\b`), "2"},
		{regexp.MustCompile(`(?i)\btwo\b`), "2"},
		{regexp.MustCompile(`(?i)\bfirst\b`), "1"},
		{regexp.MustCompile(`(?i)\bone\b`), "1"},
		{regexp.MustCompile(`(?i)\bzeroth\b`), "0"},
		{regexp.MustCompile(`(?i)\bzero\b`), "0"},
	}

	// Trailing article: detect "The X" / "A X" / "An X" at start of name.
	// Moves it to end: "The Addams Family" -> "Addams Family, The".
	// Captures: (article) (name-without-trailing-paren-block) (optional trailing paren block).
	// This preserves metadata like "(Bally, 1992)" at the end of the string.
	reLeadingArticle = regexp.MustCompile(`(?i)^(The|An|A)\s+(.*?)(\s*\(.*)?$`)

	// Inverse trailing article: detect ", The" / ", A" / ", An" suffix.
	reTrailingArticle = regexp.MustCompile(`(?i)^(.+),\s+(The|An|A)\s*$`)

	// Signal extraction: name(any) ( manufacturer(non-digit/comma/paren) , YYYY
	reSignal = regexp.MustCompile(`(?i)^(.*?)\s*\(\s*([^\d(),]+?)\s*,?\s*(\d{4})`)

	// Noise tokens (version tags, format qualifiers).
	reNoise = regexp.MustCompile(`(?i)\b(v?\d+[\d.]*|vpx|vp[xt]?|mod|update|final|beta|alpha|release|rc\d*)\b`)

	// Whitespace collapse: used in NormalizeForMatching.
	reWhitespace = regexp.MustCompile(`\s+`)
)

// stripPossessive removes possessive 's (both straight and curly apostrophe).
func stripPossessive(s string) string {
	return rePossessive.ReplaceAllString(s, "")
}

// splitCamelCase inserts spaces at CamelCase boundaries.
func splitCamelCase(s string) string {
	s = reCamelLower.ReplaceAllString(s, "$1 $2")
	s = reCamelUpper.ReplaceAllString(s, "$1 $2")
	return s
}

// convertRomanNumerals replaces whole-word Roman numerals II–XX with digits.
// Processes longest-first to avoid partial replacement.
func convertRomanNumerals(s string) string {
	for _, r := range romanNumerals {
		s = r.pat.ReplaceAllString(s, r.val)
	}
	return s
}

// convertNumberWords replaces whole-word number words (0–12, cardinals and ordinals)
// with digits. Case-insensitive. Processes longest-first.
func convertNumberWords(s string) string {
	for _, n := range numberWords {
		s = n.pat.ReplaceAllString(s, n.val)
	}
	return s
}

// applyTrailingArticle moves a leading article (The/A/An) to the end of the name
// portion, comma-separated. Any trailing parenthesized metadata block is preserved
// after the relocated article.
// Example: "The Addams Family" -> "Addams Family, The"
// Example: "The Addams Family (Bally, 1992)" -> "Addams Family, The (Bally, 1992)"
// If no leading article is present, the string is returned unchanged.
func applyTrailingArticle(s string) string {
	m := reLeadingArticle.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	// m[1] = article, m[2] = name body (without trailing paren block), m[3] = trailing paren block (may be empty)
	name := strings.TrimSpace(m[2])
	if name == "" {
		// Article followed immediately by paren block — don't transform.
		return s
	}
	article := m[1]
	suffix := m[3] // e.g. " (Bally, 1992)" or ""
	return name + ", " + article + suffix
}

// Normalize applies the full pipeline in order: possessive strip -> CamelCase
// split -> Roman numeral conversion (II..XX, I excluded) -> number word
// conversion (0..12 cardinals + ordinals) -> trailing article normalization.
// Must be called BEFORE lowercasing for Roman numeral detection to work.
func Normalize(s string, trailingArticle bool) string {
	s = stripPossessive(s)
	s = splitCamelCase(s)
	s = convertRomanNumerals(s)
	s = convertNumberWords(s)
	if trailingArticle {
		s = applyTrailingArticle(s)
	}
	return s
}

// NormalizeForMatching applies Normalize plus noise stripping (version tags
// like "v600", format suffixes like "vpx"/"mod"/"final"/"beta"/"rc1").
// Used for Path B full-catalog fuzzy fallback.
func NormalizeForMatching(s string, trailingArticle bool) string {
	s = Normalize(s, trailingArticle)
	s = reNoise.ReplaceAllString(s, " ")
	s = reWhitespace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// ExtractSignal parses a stem like "Phoenix (Williams, 1978) v600" into a Signal.
// Returns nil if the stem does not contain a (Manufacturer, YYYY) block.
func ExtractSignal(stem string) *Signal {
	// Replace underscores with spaces before matching, per MIGRATION-BRIEF §9.6.
	stem = strings.ReplaceAll(stem, "_", " ")
	m := reSignal.FindStringSubmatch(stem)
	if m == nil {
		return nil
	}
	year, err := strconv.Atoi(strings.TrimSpace(m[3]))
	if err != nil {
		year = 0
	}
	return &Signal{
		Name:         strings.TrimSpace(m[1]),
		Manufacturer: strings.TrimSpace(m[2]),
		Year:         year,
	}
}

// Canonical constructs "{name} ({manufacturer}, {year})". When trailingArticle
// is true, applies the trailing-article convention to name (e.g. "The Addams
// Family" -> "Addams Family, The").
func Canonical(name, manufacturer string, year int, trailingArticle bool) string {
	// Handle trailing-article convention on the name component.
	if trailingArticle {
		// If name already has a trailing-article suffix, keep it as-is.
		if reTrailingArticle.MatchString(name) {
			// already in trailing form — no change needed
		} else {
			// Move leading article to end if present.
			name = applyTrailingArticle(name)
		}
	} else {
		// trailingArticle=false: if name has a trailing-article suffix, move article back to front.
		tm := reTrailingArticle.FindStringSubmatch(name)
		if tm != nil {
			// tm[1] = rest, tm[2] = article
			name = tm[2] + " " + tm[1]
		}
	}
	return fmt.Sprintf("%s (%s, %d)", name, manufacturer, year)
}
