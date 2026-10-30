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

const deterministicJobFamilyParserVersionV1 = "deterministic-job-family-title-parser-v1"

type JobFamilyKey string

const (
	JobFamilyAIAlgorithm           JobFamilyKey = "ai_algorithm"
	JobFamilyAIApplication         JobFamilyKey = "ai_application"
	JobFamilyAlgorithmResearch     JobFamilyKey = "algorithm_research"
	JobFamilyBackend               JobFamilyKey = "backend"
	JobFamilyClient                JobFamilyKey = "client"
	JobFamilyDataEngineering       JobFamilyKey = "data_engineering"
	JobFamilyEngineeringManagement JobFamilyKey = "engineering_management"
	JobFamilyEngineSystems         JobFamilyKey = "engine_systems"
	JobFamilyFrontend              JobFamilyKey = "frontend"
	JobFamilyHardwareEmbedded      JobFamilyKey = "hardware_embedded"
	JobFamilyOperationsSRE         JobFamilyKey = "operations_sre"
	JobFamilyPlatformInfra         JobFamilyKey = "platform_infra"
	JobFamilySecurity              JobFamilyKey = "security"
	JobFamilyTestingQuality        JobFamilyKey = "testing_quality"
	JobFamilyToolchain             JobFamilyKey = "toolchain"
)

type JobFamilyTitleStatus string

const (
	JobFamilyClassified   JobFamilyTitleStatus = "classified"
	JobFamilyUnclassified JobFamilyTitleStatus = "unclassified"
	JobFamilyInScope      JobFamilyTitleStatus = "in_scope"
	JobFamilyInvalidTitle JobFamilyTitleStatus = "invalid_title"
	JobFamilyOutOfScope   JobFamilyTitleStatus = "out_of_scope"
	JobFamilyScopeUnknown JobFamilyTitleStatus = "scope_unknown"
)

type JobFamilyTitleEvidence struct {
	FamilyKey      JobFamilyKey
	MatchedAlias   string
	CodePointRange CodePointRange
}

// JobFamilyTitleResult is derived exclusively from one normalized title and
// reviewed project-mask ranges. Scope and source-observation statuses belong
// to the persistence/read boundary, not this pure parser.
type JobFamilyTitleResult struct {
	Status            JobFamilyTitleStatus
	PrimaryFamilyKey  JobFamilyKey
	FamilyMatchCounts map[JobFamilyKey]int
	Evidence          []JobFamilyTitleEvidence
}

type jobFamilyDictionaryAsset struct {
	Version  string                     `json:"version"`
	Families []jobFamilyDictionaryEntry `json:"families"`
}

type jobFamilyDictionaryEntry struct {
	FamilyKey   JobFamilyKey     `json:"family_key"`
	DisplayName string           `json:"display_name"`
	Enabled     bool             `json:"enabled"`
	Aliases     []jobFamilyAlias `json:"aliases"`
}

type jobFamilyAlias struct {
	Value                   string `json:"value"`
	ASCIICaseInsensitive    bool   `json:"ascii_case_insensitive"`
	ASCIIIdentifierBoundary bool   `json:"ascii_identifier_boundary"`
}

type jobFamilyTitleMatch struct {
	family    jobFamilyDictionaryEntry
	alias     jobFamilyAlias
	byteStart int
	byteEnd   int
	interval  CodePointRange
}

//go:embed job_family_dictionary_v3.json
var deterministicJobFamilyDictionaryAsset []byte

var deterministicJobFamilyDictionary struct {
	once   sync.Once
	asset  jobFamilyDictionaryAsset
	digest string
	err    error
}

func JobFamilyParserVersion() string {
	return deterministicJobFamilyParserVersionV1
}

func JobFamilyDictionaryVersion() string {
	asset, _ := loadDeterministicJobFamilyDictionary()
	return asset.Version
}

func JobFamilyDictionaryDigest() string {
	_, digest := loadDeterministicJobFamilyDictionary()
	return digest
}

// ParseJobFamilyTitle is the public pure parsing seam. It deliberately
// receives no JD body, source-project metadata, technology, or AI fields.
func ParseJobFamilyTitle(title string) (JobFamilyTitleResult, error) {
	asset, _ := loadDeterministicJobFamilyDictionary()
	if deterministicJobFamilyDictionary.err != nil {
		return JobFamilyTitleResult{}, deterministicJobFamilyDictionary.err
	}
	if !utf8.ValidString(title) {
		return JobFamilyTitleResult{}, fmt.Errorf("normalized job-family title is not valid UTF-8")
	}
	projectMasks, err := jobFamilyProjectMaskRanges(title)
	if err != nil {
		return JobFamilyTitleResult{}, fmt.Errorf("parse reviewed project-title masks: %w", err)
	}
	selected := selectJobFamilyTitleMatches(jobFamilyTitleCandidates(asset.Families, title, projectMasks))
	result := JobFamilyTitleResult{FamilyMatchCounts: make(map[JobFamilyKey]int)}
	lastEndByFamily := make(map[JobFamilyKey]int)
	for _, match := range selected {
		result.FamilyMatchCounts[match.family.FamilyKey]++
		if match.interval.End > lastEndByFamily[match.family.FamilyKey] {
			lastEndByFamily[match.family.FamilyKey] = match.interval.End
		}
		result.Evidence = append(result.Evidence, JobFamilyTitleEvidence{
			FamilyKey:      match.family.FamilyKey,
			MatchedAlias:   match.alias.Value,
			CodePointRange: match.interval,
		})
	}
	if len(result.FamilyMatchCounts) == 0 {
		result.Status = JobFamilyUnclassified
		return result, nil
	}
	result.Status = JobFamilyClassified
	for familyKey, count := range result.FamilyMatchCounts {
		if result.PrimaryFamilyKey == "" || jobFamilyPrimaryWins(
			familyKey,
			count,
			lastEndByFamily[familyKey],
			result.PrimaryFamilyKey,
			result.FamilyMatchCounts[result.PrimaryFamilyKey],
			lastEndByFamily[result.PrimaryFamilyKey],
		) {
			result.PrimaryFamilyKey = familyKey
		}
	}
	return result, nil
}

func jobFamilyPrimaryWins(candidate JobFamilyKey, candidateCount, candidateEnd int, current JobFamilyKey, currentCount, currentEnd int) bool {
	if candidateCount != currentCount {
		return candidateCount > currentCount
	}
	if candidateEnd != currentEnd {
		return candidateEnd > currentEnd
	}
	return candidate < current
}

func loadDeterministicJobFamilyDictionary() (jobFamilyDictionaryAsset, string) {
	deterministicJobFamilyDictionary.once.Do(func() {
		asset := jobFamilyDictionaryAsset{}
		if err := json.Unmarshal(deterministicJobFamilyDictionaryAsset, &asset); err != nil {
			deterministicJobFamilyDictionary.err = fmt.Errorf("decode deterministic job-family dictionary: %w", err)
			return
		}
		if err := validateJobFamilyDictionary(asset); err != nil {
			deterministicJobFamilyDictionary.err = err
			return
		}
		digest := sha256.Sum256(deterministicJobFamilyDictionaryAsset)
		deterministicJobFamilyDictionary.asset = asset
		deterministicJobFamilyDictionary.digest = hex.EncodeToString(digest[:])
	})
	return deterministicJobFamilyDictionary.asset, deterministicJobFamilyDictionary.digest
}

func validateJobFamilyDictionary(asset jobFamilyDictionaryAsset) error {
	if asset.Version == "" {
		return fmt.Errorf("deterministic job-family dictionary has no version")
	}
	expectedFamilies := map[JobFamilyKey]struct{}{
		JobFamilyAIAlgorithm: {}, JobFamilyAIApplication: {}, JobFamilyAlgorithmResearch: {}, JobFamilyBackend: {}, JobFamilyClient: {}, JobFamilyDataEngineering: {},
		JobFamilyEngineeringManagement: {}, JobFamilyEngineSystems: {}, JobFamilyFrontend: {}, JobFamilyHardwareEmbedded: {},
		JobFamilyOperationsSRE: {}, JobFamilyPlatformInfra: {}, JobFamilySecurity: {}, JobFamilyTestingQuality: {}, JobFamilyToolchain: {},
	}
	seenFamilies := make(map[JobFamilyKey]struct{}, len(asset.Families))
	type registeredAlias struct {
		familyKey JobFamilyKey
		alias     jobFamilyAlias
	}
	aliases := make([]registeredAlias, 0)
	for _, family := range asset.Families {
		if _, expected := expectedFamilies[family.FamilyKey]; !expected || family.DisplayName == "" || len(family.Aliases) == 0 {
			return fmt.Errorf("deterministic job-family dictionary has invalid family")
		}
		if _, exists := seenFamilies[family.FamilyKey]; exists {
			return fmt.Errorf("deterministic job-family dictionary repeats a family key")
		}
		seenFamilies[family.FamilyKey] = struct{}{}
		for _, alias := range family.Aliases {
			if alias.Value == "" || !utf8.ValidString(alias.Value) {
				return fmt.Errorf("deterministic job-family dictionary has invalid alias")
			}
			for _, registered := range aliases {
				if !jobFamilyAliasesOverlap(alias, registered.alias) {
					continue
				}
				if registered.familyKey == family.FamilyKey {
					return fmt.Errorf("deterministic job-family dictionary repeats an alias")
				}
				return fmt.Errorf("deterministic job-family dictionary maps one alias to multiple families")
			}
			aliases = append(aliases, registeredAlias{familyKey: family.FamilyKey, alias: alias})
		}
	}
	if len(seenFamilies) != len(expectedFamilies) {
		return fmt.Errorf("deterministic job-family dictionary does not contain every approved family")
	}
	return nil
}

// jobFamilyAliasesOverlap reports whether two declared aliases can match the
// same title fragment. This prevents case-insensitive ASCII aliases from
// acquiring context-dependent ownership through a casing-only duplicate.
func jobFamilyAliasesOverlap(left, right jobFamilyAlias) bool {
	if left.Value == right.Value {
		return true
	}
	return (left.ASCIICaseInsensitive || right.ASCIICaseInsensitive) && strings.EqualFold(left.Value, right.Value)
}

func jobFamilyProjectMaskRanges(title string) ([]CodePointRange, error) {
	projectTags, err := ParseProjectTitle(title)
	if err != nil {
		return nil, err
	}
	seen := make(map[CodePointRange]struct{})
	masks := make([]CodePointRange, 0)
	for _, assignment := range projectTags.Assignments {
		for _, evidence := range assignment.Evidence {
			if _, exists := seen[evidence.CodePointRange]; exists {
				continue
			}
			seen[evidence.CodePointRange] = struct{}{}
			masks = append(masks, evidence.CodePointRange)
		}
	}
	return masks, nil
}

func jobFamilyTitleCandidates(families []jobFamilyDictionaryEntry, title string, projectMasks []CodePointRange) []jobFamilyTitleMatch {
	candidates := make([]jobFamilyTitleMatch, 0)
	for _, family := range families {
		if !family.Enabled {
			continue
		}
		for _, alias := range family.Aliases {
			for byteStart := range title {
				byteEnd := byteStart + len(alias.Value)
				if byteEnd > len(title) || !jobFamilyAliasMatchesAt(alias, title, byteStart, byteEnd) {
					continue
				}
				interval, err := CodePointRangeFromByteOffsets(title, byteStart, byteEnd)
				if err != nil || jobFamilyRangeIsMasked(interval, projectMasks) {
					continue
				}
				candidates = append(candidates, jobFamilyTitleMatch{family: family, alias: alias, byteStart: byteStart, byteEnd: byteEnd, interval: interval})
			}
		}
	}
	return candidates
}

func jobFamilyAliasMatchesAt(alias jobFamilyAlias, title string, byteStart, byteEnd int) bool {
	segment := title[byteStart:byteEnd]
	if alias.ASCIICaseInsensitive {
		if !strings.EqualFold(segment, alias.Value) {
			return false
		}
	} else if segment != alias.Value {
		return false
	}
	return !alias.ASCIIIdentifierBoundary || (!isJobFamilyASCIIIdentifierByte(byteBefore(title, byteStart)) && !isJobFamilyASCIIIdentifierByte(byteAfter(title, byteEnd)))
}

func isJobFamilyASCIIIdentifierByte(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9') || value == '_'
}

func jobFamilyRangeIsMasked(interval CodePointRange, masks []CodePointRange) bool {
	for _, mask := range masks {
		if interval.Start < mask.End && mask.Start < interval.End {
			return true
		}
	}
	return false
}

func selectJobFamilyTitleMatches(candidates []jobFamilyTitleMatch) []jobFamilyTitleMatch {
	byFamily := make(map[JobFamilyKey][]jobFamilyTitleMatch)
	for _, candidate := range candidates {
		byFamily[candidate.family.FamilyKey] = append(byFamily[candidate.family.FamilyKey], candidate)
	}
	selected := make([]jobFamilyTitleMatch, 0, len(candidates))
	for _, familyCandidates := range byFamily {
		sort.Slice(familyCandidates, func(left, right int) bool {
			leftLength := familyCandidates[left].byteEnd - familyCandidates[left].byteStart
			rightLength := familyCandidates[right].byteEnd - familyCandidates[right].byteStart
			if leftLength != rightLength {
				return leftLength > rightLength
			}
			if familyCandidates[left].alias.Value != familyCandidates[right].alias.Value {
				return familyCandidates[left].alias.Value < familyCandidates[right].alias.Value
			}
			return familyCandidates[left].byteStart < familyCandidates[right].byteStart
		})
		familySelected := make([]jobFamilyTitleMatch, 0, len(familyCandidates))
		for _, candidate := range familyCandidates {
			overlaps := false
			for _, existing := range familySelected {
				if candidate.byteStart < existing.byteEnd && existing.byteStart < candidate.byteEnd {
					overlaps = true
					break
				}
			}
			if !overlaps {
				familySelected = append(familySelected, candidate)
			}
		}
		selected = append(selected, familySelected...)
	}
	sort.Slice(selected, func(left, right int) bool {
		if selected[left].family.FamilyKey != selected[right].family.FamilyKey {
			return selected[left].family.FamilyKey < selected[right].family.FamilyKey
		}
		if selected[left].interval.Start != selected[right].interval.Start {
			return selected[left].interval.Start < selected[right].interval.Start
		}
		if selected[left].interval.End != selected[right].interval.End {
			return selected[left].interval.End < selected[right].interval.End
		}
		return selected[left].alias.Value < selected[right].alias.Value
	})
	return selected
}
