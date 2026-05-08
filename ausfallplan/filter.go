package ausfallplan

import (
	"regexp"
	"strings"
)

// fragmentRe matches an anchored "year + optional-space + letter" fragment,
// e.g. "3d", "3 D", "6 b", "4e". Letter class is broad ([a-z]) — the tokenizer
// just identifies syntactic class tokens; the blacklist decides relevance.
// This catches unusual letters like Stechlinsee's 4e (a fifth 4th-grade group).
// Anchored matching keeps "3a Englisch" (extra text) from matching.
var fragmentRe = regexp.MustCompile(`(?i)^([1-6])\s*([a-z])$`)

// letterRe matches an anchored single letter (used for year-carry). Same
// reasoning as fragmentRe — broad letter class, blacklist decides relevance.
var letterRe = regexp.MustCompile(`(?i)^([a-z])$`)

// classTokens holds the result of tokenizing a Class field.
type classTokens struct {
	recognized      []string // e.g. ["3d", "6b"]
	hasUnrecognized bool     // true if any fragment didn't match either regex
}

// extractClasses tokenizes the Class field by comma. Each fragment is matched
// against the anchored year+letter pattern; if that fails, the year-carry pattern
// (single letter, using the most recently seen year) is tried. Anything that
// matches neither is flagged as unrecognized.
//
// Year-carry: "6 a, b, c" yields ["6a","6b","6c"]. The carry resets per field
// (not between entries). This is non-trivial — a single letter after a full token
// inherits the year from the preceding token in the same comma-separated list.
func extractClasses(class string) classTokens {
	fragments := strings.Split(class, ",")

	var recognized []string
	var hasUnrecognized bool
	var lastYear string

	for _, frag := range fragments {
		frag = strings.TrimSpace(frag)
		if frag == "" {
			continue
		}

		if m := fragmentRe.FindStringSubmatch(frag); m != nil {
			// Full year+letter token.
			year := m[1]
			letter := strings.ToLower(m[2])
			recognized = append(recognized, year+letter)
			lastYear = year
		} else if m := letterRe.FindStringSubmatch(frag); m != nil && lastYear != "" {
			// Single-letter fragment with a carried year.
			letter := strings.ToLower(m[1])
			recognized = append(recognized, lastYear+letter)
		} else {
			// Unrecognized fragment — keep the entry.
			hasUnrecognized = true
		}
	}

	return classTokens{recognized: recognized, hasUnrecognized: hasUnrecognized}
}

// normalizeClass lowercases and strips all spaces so "3 D" and "3d" both become "3d".
func normalizeClass(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", ""))
}

// Filter drops entries whose Class field consists exclusively of blacklisted class
// tokens. See the brief for full semantics.
//
// blacklist values are normalized (lowercased, spaces stripped) before comparison;
// an empty blacklist results in zero filtering (all entries kept).
func Filter(entries []Entry, blacklist []string) []Entry {
	if len(blacklist) == 0 {
		return entries
	}

	// Build a normalized set for O(1) lookup.
	blacklistSet := make(map[string]struct{}, len(blacklist))
	for _, b := range blacklist {
		blacklistSet[normalizeClass(b)] = struct{}{}
	}

	result := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if shouldKeep(e, blacklistSet) {
			result = append(result, e)
		}
	}
	return result
}

// shouldKeep returns true if the entry should be retained (not filtered out).
// An entry is dropped only when:
//   - it has ≥1 recognized token, AND
//   - hasUnrecognized is false, AND
//   - every recognized token is in the blacklist.
func shouldKeep(e Entry, blacklistSet map[string]struct{}) bool {
	tokens := extractClasses(e.Class)

	if len(tokens.recognized) == 0 {
		// No recognized tokens → keep (covers empty, free-text, "alle Klassen", etc.)
		return true
	}
	if tokens.hasUnrecognized {
		// At least one unrecognized fragment → keep (over-notify on ambiguity).
		return true
	}

	// All fragments were recognized. Drop only if every token is blacklisted.
	for _, tok := range tokens.recognized {
		if _, inList := blacklistSet[tok]; !inList {
			return true // at least one recognized token not in blacklist → keep
		}
	}
	return false // every recognized token is blacklisted → drop
}
