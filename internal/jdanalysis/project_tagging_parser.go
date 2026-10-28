package jdanalysis

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const deterministicProjectTagParserVersionV1 = "deterministic-project-title-parser-v1"

type ProjectTagClassification string

const (
	ProjectTagNamedGame       ProjectTagClassification = "named_game"
	ProjectTagSourceDirection ProjectTagClassification = "source_direction"
)

type ProjectTitleEvidence struct {
	MatchedKeyword string
	CodePointRange CodePointRange
}

type ProjectTagAssignment struct {
	ProjectKey     string
	DisplayName    string
	Classification ProjectTagClassification
	Evidence       []ProjectTitleEvidence
}

// ProjectTitleTagResult deliberately carries no source-project, JD-body,
// technology, or AI field: those inputs cannot affect title assignment.
type ProjectTitleTagResult struct {
	Assignments []ProjectTagAssignment
}

type projectDictionaryAsset struct {
	Version string                   `json:"version"`
	Entries []projectDictionaryEntry `json:"entries"`
}

type projectDictionaryEntry struct {
	ProjectKey     string                   `json:"project_key"`
	DisplayName    string                   `json:"display_name"`
	Classification ProjectTagClassification `json:"classification"`
	Enabled        bool                     `json:"enabled"`
	Keywords       []projectKeyword         `json:"keywords"`
}

type projectKeyword struct {
	Value                   string `json:"value"`
	ASCIICaseInsensitive    bool   `json:"ascii_case_insensitive"`
	ASCIIIdentifierBoundary bool   `json:"ascii_identifier_boundary"`
}

type projectTitleMatch struct {
	entry     projectDictionaryEntry
	keyword   projectKeyword
	byteStart int
	byteEnd   int
}

//go:embed project_dictionary_v1.json
var deterministicProjectDictionaryAsset []byte

var deterministicProjectDictionary struct {
	once   sync.Once
	asset  projectDictionaryAsset
	digest string
	err    error
}

// ProjectTagParserVersion is the immutable parser identity stored beside each
// title-level result.
func ProjectTagParserVersion() string {
	return deterministicProjectTagParserVersionV1
}

// ProjectTagDictionaryVersion identifies the reviewed embedded project
// dictionary. Updating its contents requires a new version and digest.
func ProjectTagDictionaryVersion() string {
	asset, _ := loadDeterministicProjectDictionary()
	return asset.Version
}

// ProjectTagDictionaryDigest returns the exact asset digest used by this
// parser, keeping title assignments reproducible across dictionary revisions.
func ProjectTagDictionaryDigest() string {
	_, digest := loadDeterministicProjectDictionary()
	return digest
}

// ParseProjectTitle matches only a normalized title. It is deliberately
// unable to receive source project metadata, JD body fields, or model output.
func ParseProjectTitle(title string) (ProjectTitleTagResult, error) {
	asset, _ := loadDeterministicProjectDictionary()
	if deterministicProjectDictionary.err != nil {
		return ProjectTitleTagResult{}, deterministicProjectDictionary.err
	}
	if !utf8.ValidString(title) {
		return ProjectTitleTagResult{}, fmt.Errorf("normalized project title is not valid UTF-8")
	}
	matches := selectProjectTitleMatches(projectTitleCandidates(asset.Entries, title))
	assignments := make([]ProjectTagAssignment, 0, len(matches))
	assignmentIndexes := make(map[string]int)
	for _, match := range matches {
		interval, err := CodePointRangeFromByteOffsets(title, match.byteStart, match.byteEnd)
		if err != nil {
			return ProjectTitleTagResult{}, fmt.Errorf("convert project title evidence coordinate: %w", err)
		}
		index, exists := assignmentIndexes[match.entry.ProjectKey]
		if !exists {
			index = len(assignments)
			assignmentIndexes[match.entry.ProjectKey] = index
			assignments = append(assignments, ProjectTagAssignment{
				ProjectKey:     match.entry.ProjectKey,
				DisplayName:    match.entry.DisplayName,
				Classification: match.entry.Classification,
			})
		}
		assignments[index].Evidence = append(assignments[index].Evidence, ProjectTitleEvidence{
			MatchedKeyword: match.keyword.Value,
			CodePointRange: interval,
		})
	}
	return ProjectTitleTagResult{Assignments: assignments}, nil
}

func loadDeterministicProjectDictionary() (projectDictionaryAsset, string) {
	deterministicProjectDictionary.once.Do(func() {
		asset := projectDictionaryAsset{}
		if err := json.Unmarshal(deterministicProjectDictionaryAsset, &asset); err != nil {
			deterministicProjectDictionary.err = fmt.Errorf("decode deterministic project dictionary: %w", err)
			return
		}
		if err := validateProjectDictionary(asset); err != nil {
			deterministicProjectDictionary.err = err
			return
		}
		digest := sha256.Sum256(deterministicProjectDictionaryAsset)
		deterministicProjectDictionary.asset = asset
		deterministicProjectDictionary.digest = hex.EncodeToString(digest[:])
	})
	return deterministicProjectDictionary.asset, deterministicProjectDictionary.digest
}

func validateProjectDictionary(asset projectDictionaryAsset) error {
	if asset.Version == "" || len(asset.Entries) == 0 {
		return fmt.Errorf("deterministic project dictionary is incomplete")
	}
	projectKeys := make(map[string]struct{}, len(asset.Entries))
	for _, entry := range asset.Entries {
		if entry.ProjectKey == "" || entry.DisplayName == "" || !validProjectTagClassification(entry.Classification) || len(entry.Keywords) == 0 {
			return fmt.Errorf("deterministic project dictionary has invalid entry")
		}
		if _, exists := projectKeys[entry.ProjectKey]; exists {
			return fmt.Errorf("deterministic project dictionary repeats a project key")
		}
		projectKeys[entry.ProjectKey] = struct{}{}
		keywords := make(map[string]struct{}, len(entry.Keywords))
		for _, keyword := range entry.Keywords {
			if keyword.Value == "" || !utf8.ValidString(keyword.Value) {
				return fmt.Errorf("deterministic project dictionary has invalid keyword")
			}
			if _, exists := keywords[keyword.Value]; exists {
				return fmt.Errorf("deterministic project dictionary repeats an entry keyword")
			}
			keywords[keyword.Value] = struct{}{}
		}
	}
	return nil
}

func validProjectTagClassification(classification ProjectTagClassification) bool {
	return classification == ProjectTagNamedGame || classification == ProjectTagSourceDirection
}

func projectTitleCandidates(entries []projectDictionaryEntry, title string) []projectTitleMatch {
	candidates := make([]projectTitleMatch, 0)
	for _, entry := range entries {
		if !entry.Enabled {
			continue
		}
		for _, keyword := range entry.Keywords {
			for byteStart := range title {
				byteEnd := byteStart + len(keyword.Value)
				if byteEnd > len(title) || !projectKeywordMatchesAt(keyword, title, byteStart, byteEnd) {
					continue
				}
				candidates = append(candidates, projectTitleMatch{entry: entry, keyword: keyword, byteStart: byteStart, byteEnd: byteEnd})
			}
		}
	}
	return candidates
}

func projectKeywordMatchesAt(keyword projectKeyword, title string, byteStart, byteEnd int) bool {
	segment := title[byteStart:byteEnd]
	if keyword.ASCIICaseInsensitive {
		if !strings.EqualFold(segment, keyword.Value) {
			return false
		}
	} else if segment != keyword.Value {
		return false
	}
	if !keyword.ASCIIIdentifierBoundary {
		return true
	}
	return !isProjectASCIIIdentifierByte(byteBefore(title, byteStart)) && !isProjectASCIIIdentifierByte(byteAfter(title, byteEnd))
}

func isProjectASCIIIdentifierByte(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9') || value == '_'
}

func selectProjectTitleMatches(candidates []projectTitleMatch) []projectTitleMatch {
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].byteStart != candidates[right].byteStart {
			return candidates[left].byteStart < candidates[right].byteStart
		}
		leftLength := candidates[left].byteEnd - candidates[left].byteStart
		rightLength := candidates[right].byteEnd - candidates[right].byteStart
		if leftLength != rightLength {
			return leftLength > rightLength
		}
		if candidates[left].entry.ProjectKey != candidates[right].entry.ProjectKey {
			return candidates[left].entry.ProjectKey < candidates[right].entry.ProjectKey
		}
		return candidates[left].keyword.Value < candidates[right].keyword.Value
	})
	selected := make([]projectTitleMatch, 0, len(candidates))
	for _, candidate := range candidates {
		overlaps := false
		for _, existing := range selected {
			if candidate.byteStart < existing.byteEnd && existing.byteStart < candidate.byteEnd {
				overlaps = true
				break
			}
		}
		if !overlaps {
			selected = append(selected, candidate)
		}
	}
	return selected
}
