package jdanalysis

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

const (
	aiOutputAuditDiagnosticSampleSize = 5
	aiTrialSampleSize                 = 30
)

// AIBodySamplingMetric is a non-text body metric used to freeze a trial
// sample. It deliberately has no source field, snippet, title, or metadata.
type AIBodySamplingMetric struct {
	BodyContentVersionID string
	BodyLengthCodePoints int
	CompleteFieldCount   int
	VerifiedMentionCount int
}

// SelectAITrialSampleV1 deterministically selects exactly 30 body-version IDs
// using non-text length, completeness, and mention-density strata. It never
// silently expands a small candidate set or returns source content.
func SelectAITrialSampleV1(candidates []AIBodySamplingMetric) ([]string, error) {
	return selectAIBodySampleV1(candidates, aiTrialSampleSize, "ai-trial-sample-v1")
}

// SelectAIOutputAuditDiagnosticSampleV1 freezes exactly five non-text body
// IDs for the one fixed Plus-only contract diagnostic. It is deliberately a
// separate sample identity from the 30-body release trial.
func SelectAIOutputAuditDiagnosticSampleV1(candidates []AIBodySamplingMetric) ([]string, error) {
	return selectAIBodySampleV1(candidates, aiOutputAuditDiagnosticSampleSize, "ai-output-audit-diagnostic-sample-v1")
}

func selectAIBodySampleV1(candidates []AIBodySamplingMetric, sampleSize int, samplingVersion string) ([]string, error) {
	if len(candidates) < sampleSize || sampleSize < 1 {
		return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	strata := make(map[string][]AIBodySamplingMetric)
	seenIDs := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate.BodyContentVersionID == "" || candidate.BodyLengthCodePoints < 0 || candidate.CompleteFieldCount < 0 || candidate.CompleteFieldCount > 3 || candidate.VerifiedMentionCount < 0 {
			return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		if _, duplicate := seenIDs[candidate.BodyContentVersionID]; duplicate {
			return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		seenIDs[candidate.BodyContentVersionID] = struct{}{}
		key := aiSamplingStratum(candidate)
		strata[key] = append(strata[key], candidate)
	}
	keys := make([]string, 0, len(strata))
	for key := range strata {
		keys = append(keys, key)
		sort.Slice(strata[key], func(left, right int) bool {
			return aiSamplingRankForVersion(samplingVersion, strata[key][left].BodyContentVersionID) < aiSamplingRankForVersion(samplingVersion, strata[key][right].BodyContentVersionID)
		})
	}
	sort.Strings(keys)

	selected := make([]string, 0, sampleSize)
	nextByStratum := make(map[string]int, len(keys))
	for len(selected) < sampleSize {
		selectedThisPass := false
		for _, key := range keys {
			next := nextByStratum[key]
			if next >= len(strata[key]) {
				continue
			}
			selected = append(selected, strata[key][next].BodyContentVersionID)
			nextByStratum[key] = next + 1
			selectedThisPass = true
			if len(selected) == sampleSize {
				break
			}
		}
		if !selectedThisPass {
			return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
	}
	return selected, nil
}

func aiSamplingRank(bodyContentVersionID string) string {
	return aiSamplingRankForVersion("ai-trial-sample-v1", bodyContentVersionID)
}

func aiSamplingRankForVersion(samplingVersion, bodyContentVersionID string) string {
	digest := sha256.Sum256([]byte(samplingVersion + "|" + bodyContentVersionID))
	return hex.EncodeToString(digest[:])
}

func aiSamplingStratum(metric AIBodySamplingMetric) string {
	return aiBodyLengthBand(metric.BodyLengthCodePoints) + ":" +
		string(rune('0'+metric.CompleteFieldCount)) + ":" +
		aiMentionDensityBand(metric.BodyLengthCodePoints, metric.VerifiedMentionCount)
}

func aiBodyLengthBand(length int) string {
	switch {
	case length < 500:
		return "short"
	case length < 2000:
		return "medium"
	default:
		return "long"
	}
}

func aiMentionDensityBand(length, count int) string {
	if count == 0 {
		return "none"
	}
	if length == 0 || count*1000 > length*2 {
		return "dense"
	}
	return "sparse"
}
