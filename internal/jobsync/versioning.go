package jobsync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// These identifiers are stored alongside the persisted hashes. Changing a
// normalizer is a data-contract change, not an in-place rewrite of an old
// version.
const (
	MetadataNormalizationVersion = "source-metadata-v1"
	BodyNormalizationVersion     = "jd-body-v1"
)

type normalizedBodyContent struct {
	Description string `json:"description"`
	JobRequire  string `json:"job_require"`
	Addition    string `json:"addition"`
}

// normalizeSourceMetadata preserves the distinction between an absent key,
// JSON null, empty values, valid source values, and invalid types.  That is
// essential: turning all of them into an empty string would hide a source
// contract change and incorrectly reuse a metadata version.
func normalizeSourceMetadata(detail, listItem map[string]any) map[string]any {
	return map[string]any{
		"title":             normalizedTextField(selectFirstPresent(detail, listItem, "title", "jobName", "name")),
		"locations":         normalizedLocationsField(selectFirstPresent(detail, listItem, "addressDetailList")),
		"official_category": normalizedPair(selectFirstPresent(detail, listItem, "competencyTypeId"), selectFirstPresent(detail, listItem, "competencyType")),
		"work_nature":       normalizedPair(selectFirstPresent(detail, listItem, "jobNatureId", "natureId"), selectFirstPresent(detail, listItem, "jobNature", "nature")),
		"hire_type":         normalizedPair(selectFirstPresent(detail, listItem, "hireType"), selectFirstPresent(detail, listItem, "hireTypeName")),
		"hurry":             normalizedBooleanField(selectFirstPresent(detail, listItem, "hurry")),
		"source_project":    normalizedPair(selectFirstPresent(detail, listItem, "projectId"), selectFirstPresent(detail, listItem, "projectName")),
		"source_object":     normalizedPair(selectFirstPresent(detail, listItem, "objectId"), selectFirstPresent(detail, listItem, "objectName")),
		"source_tags":       normalizedTagsField(selectFirstPresent(detail, listItem, "tagList")),
	}
}

func normalizeBodyContent(detail, listItem map[string]any) normalizedBodyContent {
	return normalizedBodyContent{
		Description: normalizedTextForBody(selectFirstPresent(detail, listItem, "description")),
		JobRequire:  normalizedTextForBody(selectFirstPresent(detail, listItem, "jobRequire")),
		Addition:    normalizedTextForBody(selectFirstPresent(detail, listItem, "addition")),
	}
}

type selectedSourceField struct {
	value   any
	present bool
}

// selectFirstPresent intentionally does not skip null or empty values.  A
// detail response that explicitly says null is observably different from one
// that omits the key and falls back to the list response.
func selectFirstPresent(detail, listItem map[string]any, keys ...string) selectedSourceField {
	for _, source := range []map[string]any{detail, listItem} {
		for _, key := range keys {
			if raw, ok := source[key]; ok {
				return selectedSourceField{value: raw, present: true}
			}
		}
	}
	return selectedSourceField{}
}

func normalizedTextForBody(field selectedSourceField) string {
	if !field.present {
		return ""
	}
	return normalizedString(field.value)
}

func normalizedTextField(field selectedSourceField) map[string]any {
	if !field.present {
		return map[string]any{"state": "absent"}
	}
	if field.value == nil {
		return map[string]any{"state": "null"}
	}
	value, ok := field.value.(string)
	if !ok {
		return map[string]any{"state": invalidTypeState(field.value)}
	}
	normalized := normalizedString(value)
	if normalized == "" {
		return map[string]any{"state": "empty_string"}
	}
	return map[string]any{"state": "string", "value": normalized}
}

func normalizedScalarField(field selectedSourceField) map[string]any {
	if !field.present {
		return map[string]any{"state": "absent"}
	}
	if field.value == nil {
		return map[string]any{"state": "null"}
	}
	switch value := field.value.(type) {
	case string:
		normalized := normalizedString(value)
		if normalized == "" {
			return map[string]any{"state": "empty_string"}
		}
		return map[string]any{"state": "string", "value": normalized}
	case float64:
		return map[string]any{"state": "number", "value": value}
	case bool:
		return map[string]any{"state": "boolean", "value": value}
	default:
		return map[string]any{"state": invalidTypeState(field.value)}
	}
}

func normalizedBooleanField(field selectedSourceField) map[string]any {
	if !field.present {
		return map[string]any{"state": "absent"}
	}
	if field.value == nil {
		return map[string]any{"state": "null"}
	}
	value, ok := field.value.(bool)
	if !ok {
		return map[string]any{"state": invalidTypeState(field.value)}
	}
	return map[string]any{"state": "boolean", "value": value}
}

func normalizedPair(id, name selectedSourceField) map[string]any {
	return map[string]any{
		"id":   normalizedScalarField(id),
		"name": normalizedTextField(name),
	}
}

func normalizedLocationsField(field selectedSourceField) map[string]any {
	if !field.present {
		return map[string]any{"state": "absent"}
	}
	if field.value == nil {
		return map[string]any{"state": "null"}
	}
	values, ok := field.value.([]any)
	if !ok {
		return map[string]any{"state": invalidTypeState(field.value)}
	}
	if len(values) == 0 {
		return map[string]any{"state": "empty_array"}
	}
	unique := make(map[string]map[string]any, len(values))
	for _, raw := range values {
		item, ok := raw.(map[string]any)
		if !ok {
			invalid := map[string]any{"state": invalidTypeState(raw)}
			unique[string(canonicalJSON(invalid))] = invalid
			continue
		}
		pair := normalizedPair(selectFirstPresent(item, nil, "addressId", "id"), selectFirstPresent(item, nil, "addressDetail", "name", "city"))
		unique[string(canonicalJSON(pair))] = pair
	}
	return sortedSetPayload(unique)
}

func normalizedTagsField(field selectedSourceField) map[string]any {
	if !field.present {
		return map[string]any{"state": "absent"}
	}
	if field.value == nil {
		return map[string]any{"state": "null"}
	}
	values, ok := field.value.([]any)
	if !ok {
		return map[string]any{"state": invalidTypeState(field.value)}
	}
	if len(values) == 0 {
		return map[string]any{"state": "empty_array"}
	}
	unique := make(map[string]map[string]any, len(values))
	for _, raw := range values {
		item := normalizedTextField(selectedSourceField{value: raw, present: true})
		unique[string(canonicalJSON(item))] = item
	}
	return sortedSetPayload(unique)
}

func sortedSetPayload(unique map[string]map[string]any) map[string]any {
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]any, 0, len(keys))
	for _, key := range keys {
		items = append(items, unique[key])
	}
	return map[string]any{"state": "array", "items": items}
}

func invalidTypeState(value any) string {
	return fmt.Sprintf("invalid_%T", value)
}

func fingerprint(version string, payload any) string {
	return textSHA256(string(canonicalJSON(struct {
		Version string `json:"version"`
		Payload any    `json:"payload"`
	}{Version: version, Payload: payload})))
}

func textSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
