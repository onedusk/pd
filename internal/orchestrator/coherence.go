package orchestrator

import (
	"fmt"
	"regexp"
	"strings"
)

// codeBlockRe matches fenced code blocks (``` ... ```).
var codeBlockRe = regexp.MustCompile("(?s)```.*?```")

// depVersionRe matches patterns like "React 18.2", "Go 1.22.3", "node v20.x",
// "PostgreSQL 16", or any word followed by an optional 'v' and a version number.
var depVersionRe = regexp.MustCompile(`(?i)\b([A-Za-z][A-Za-z0-9_.-]*)\s+v?(\d+\.\d+(?:\.\d+)?(?:\.x)?)\b`)

// CheckCoherence performs a lightweight cross-section consistency scan.
// It extracts dependency mentions with version numbers from each section,
// builds a map of dependency name to version to section, and flags any
// dependency that appears with different versions across sections.
// Content inside fenced code blocks is excluded to avoid false positives.
func CheckCoherence(sections []Section) ([]CoherenceIssue, error) {
	// depVersions maps normalized dependency name -> version -> list of section names.
	depVersions := make(map[string]map[string][]string)

	for _, sec := range sections {
		// Strip fenced code blocks to avoid false positives.
		cleaned := codeBlockRe.ReplaceAllString(sec.Content, "")

		matches := depVersionRe.FindAllStringSubmatch(cleaned, -1)
		// Deduplicate within a single section so the same mention
		// doesn't produce self-conflicts.
		sectionSeen := make(map[string]map[string]bool)

		for _, match := range matches {
			name := strings.ToLower(match[1])
			version := match[2]

			if sectionSeen[name] == nil {
				sectionSeen[name] = make(map[string]bool)
			}
			if sectionSeen[name][version] {
				continue
			}
			sectionSeen[name][version] = true

			if depVersions[name] == nil {
				depVersions[name] = make(map[string][]string)
			}
			depVersions[name][version] = append(depVersions[name][version], sec.Name)
		}
	}

	// Find dependencies with conflicting versions across sections.
	var issues []CoherenceIssue
	for dep, versions := range depVersions {
		if len(versions) <= 1 {
			continue
		}
		// Collect all version-section pairs for the conflict description.
		var pairs []struct {
			version  string
			sections []string
		}
		for v, secs := range versions {
			pairs = append(pairs, struct {
				version  string
				sections []string
			}{version: v, sections: secs})
		}

		// Generate a CoherenceIssue for each pair of conflicting versions.
		for i := 0; i < len(pairs); i++ {
			for j := i + 1; j < len(pairs); j++ {
				// Use the first section from each version group.
				issues = append(issues, CoherenceIssue{
					SectionA: pairs[i].sections[0],
					SectionB: pairs[j].sections[0],
					Description: fmt.Sprintf(
						"dependency %q has conflicting versions: %s (in %s) vs %s (in %s)",
						dep,
						pairs[i].version, strings.Join(pairs[i].sections, ", "),
						pairs[j].version, strings.Join(pairs[j].sections, ", "),
					),
				})
			}
		}
	}

	return issues, nil
}
