// Package jobsync contains the durable, one-shot daily job synchronization
// workflow.  It deliberately has no HTTP handlers: the worker is the only
// entry point that mutates synchronization history.
package jobsync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"time"
)

const SourceCompanyMihoyo = "mihoyo"

var (
	htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)
	spacePattern   = regexp.MustCompile(`\s+`)
)

// Scope identifies a comparable source filter.  Job identity intentionally
// does not include this scope; a job can appear in more than one scope later.
type Scope struct {
	SourceCompany string
	Filter        map[string]any
	FilterSHA256  string
}

// MihoyoTechnicalScope is intentionally fixed for this delivery.  Keep this
// in lockstep with collector/mihoyo_collector.py's public FILTER constant.
func MihoyoTechnicalScope() Scope {
	filter := map[string]any{
		"channelDetailIds": []any{float64(1)},
		"competencyTypes":  []any{"1"},
		"hireType":         float64(0),
	}
	encoded := canonicalJSON(filter)
	sum := sha256.Sum256(encoded)
	return Scope{SourceCompany: SourceCompanyMihoyo, Filter: filter, FilterSHA256: hex.EncodeToString(sum[:])}
}

// TriggerKind records why an otherwise identical synchronization was started.
type TriggerKind string

const (
	TriggerInitialization TriggerKind = "initialization"
	TriggerScheduled      TriggerKind = "scheduled"
	TriggerRetry          TriggerKind = "retry"
)

func (trigger TriggerKind) Valid() bool {
	return trigger == TriggerInitialization || trigger == TriggerScheduled || trigger == TriggerRetry
}

// Observation is a validated, normalized list/detail observation. RawFields
// is stored once per distinct JD version; full per-run source evidence belongs
// in COS rather than in PostgreSQL.
type Observation struct {
	SourceCompany string
	SourceJobID   string
	ContentSHA256 string
	RawFields     json.RawMessage
	DisplayFields json.RawMessage
	ObservedAt    time.Time
}

type canonicalBusinessContent struct {
	Title        string   `json:"title"`
	Locations    []string `json:"locations"`
	Category     string   `json:"category"`
	Nature       string   `json:"nature"`
	Project      string   `json:"project"`
	Description  string   `json:"description"`
	Requirements string   `json:"requirements"`
	Additional   string   `json:"additional"`
}

// NormalizeObservation converts a raw detail response and its corresponding
// list item into the fixed business representation used for version hashing.
// Volatile transport data is deliberately absent from canonicalBusinessContent.
func NormalizeObservation(sourceCompany string, listItem, detail map[string]any, observedAt time.Time) (Observation, error) {
	if sourceCompany == "" {
		return Observation{}, fmt.Errorf("source company is required")
	}
	sourceJobID := normalizedString(value(detail, "id"))
	if sourceJobID == "" {
		sourceJobID = normalizedString(value(listItem, "id"))
	}
	if sourceJobID == "" {
		return Observation{}, fmt.Errorf("detail response does not contain a job id")
	}

	content := canonicalBusinessContent{
		Title:        firstNormalized(detail, listItem, "title", "jobName", "name"),
		Locations:    normalizedLocations(detail, listItem),
		Category:     firstNormalized(detail, listItem, "category", "competencyType", "positionCategory"),
		Nature:       firstNormalized(detail, listItem, "hireTypeName", "jobNature", "nature"),
		Project:      firstNormalized(detail, listItem, "projectName", "project", "businessLine"),
		Description:  firstNormalized(detail, listItem, "description", "jobDescription", "responsibility"),
		Requirements: firstNormalized(detail, listItem, "jobRequire", "requirements", "requirement"),
		Additional:   firstNormalized(detail, listItem, "addition", "additional", "remark"),
	}
	if content.Title == "" || content.Description == "" || content.Requirements == "" {
		return Observation{}, fmt.Errorf("detail response is missing a required business field for job %s", sourceJobID)
	}

	display := canonicalJSON(content)
	sum := sha256.Sum256(display)
	raw, err := json.Marshal(detail)
	if err != nil {
		return Observation{}, fmt.Errorf("encode raw fields: %w", err)
	}
	return Observation{
		SourceCompany: sourceCompany,
		SourceJobID:   sourceJobID,
		ContentSHA256: hex.EncodeToString(sum[:]),
		RawFields:     raw,
		DisplayFields: display,
		ObservedAt:    observedAt.UTC(),
	}, nil
}

func canonicalJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("canonical JSON value is not serializable: %v", err))
	}
	return encoded
}

func value(source map[string]any, key string) any {
	if source == nil {
		return nil
	}
	return source[key]
}

func firstNormalized(first, second map[string]any, keys ...string) string {
	for _, key := range keys {
		if normalized := normalizedString(value(first, key)); normalized != "" {
			return normalized
		}
		if normalized := normalizedString(value(second, key)); normalized != "" {
			return normalized
		}
	}
	return ""
}

func normalizedString(raw any) string {
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	value = html.UnescapeString(value)
	value = htmlTagPattern.ReplaceAllString(value, " ")
	return strings.TrimSpace(spacePattern.ReplaceAllString(strings.ReplaceAll(value, "\r\n", "\n"), " "))
}

func normalizedLocations(detail, listItem map[string]any) []string {
	seen := map[string]struct{}{}
	collectLocations(seen, value(detail, "addressDetailList"))
	collectLocations(seen, value(listItem, "addressDetailList"))
	for _, key := range []string{"address", "city", "workLocation", "location"} {
		if location := firstNormalized(detail, listItem, key); location != "" {
			seen[location] = struct{}{}
		}
	}
	locations := make([]string, 0, len(seen))
	for location := range seen {
		locations = append(locations, location)
	}
	sort.Strings(locations)
	return locations
}

func collectLocations(seen map[string]struct{}, raw any) {
	values, ok := raw.([]any)
	if !ok {
		return
	}
	for _, rawValue := range values {
		item, ok := rawValue.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"addressDetail", "city", "name"} {
			if location := normalizedString(item[key]); location != "" {
				seen[location] = struct{}{}
			}
		}
	}
}
