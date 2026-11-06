package jdserving

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var comparisonWhitespace = regexp.MustCompile(`[ \t]+`)
var bodyUnitMarker = regexp.MustCompile(`^\s*(?:\d{1,2}[、.)]|[-*•▪])\s*`)

const comparisonMaximumEditDistance = 2048

type comparisonDimension struct {
	field      string
	historical string
	latest     string
	equal      bool
}

func compareJobDetails(historical, latest JobDetailResponse) JobComparison {
	comparison := JobComparison{Status: ComparisonStatusAvailable}
	dimensions := []comparisonDimension{
		{field: "岗位名称", historical: historical.Title, latest: latest.Title, equal: historical.Title == latest.Title},
		{field: "工作地区", historical: formatStringSet(historical.Locations), latest: formatStringSet(latest.Locations), equal: equalStringSet(historical.Locations, latest.Locations)},
		{field: "主岗位族", historical: formatFilterOption(historical.MainJobFamily), latest: formatFilterOption(latest.MainJobFamily), equal: equalFilterOption(historical.MainJobFamily, latest.MainJobFamily)},
		{field: "副岗位族", historical: formatFilterOptions(historical.OtherJobFamilies), latest: formatFilterOptions(latest.OtherJobFamilies), equal: equalFilterOptions(historical.OtherJobFamilies, latest.OtherJobFamilies)},
		{field: "学历要求", historical: formatEducationRequirement(historical.Education), latest: formatEducationRequirement(latest.Education), equal: equalEducation(historical.Education, latest.Education)},
		{field: "最低工作年限", historical: formatExperienceRequirement(historical.Experience), latest: formatExperienceRequirement(latest.Experience), equal: equalExperience(historical.Experience, latest.Experience)},
		{field: "技术关键词", historical: formatFilterOptions(historical.TechnologyKeywords), latest: formatFilterOptions(latest.TechnologyKeywords), equal: equalFilterOptions(historical.TechnologyKeywords, latest.TechnologyKeywords)},
		{field: "项目", historical: formatFilterOptions(historical.Projects), latest: formatFilterOptions(latest.Projects), equal: equalFilterOptions(historical.Projects, latest.Projects)},
		{field: "紧急招聘", historical: formatBoolean(historical.IsUrgent), latest: formatBoolean(latest.IsUrgent), equal: historical.IsUrgent == latest.IsUrgent},
	}
	for _, dimension := range dimensions {
		if !dimension.equal {
			comparison.DimensionChanges = append(comparison.DimensionChanges, ComparisonDimensionChange{
				Field: dimension.field, Historical: dimension.historical, Latest: dimension.latest,
			})
		}
	}

	sections := []struct {
		section BodySection
		label   string
		history string
		latest  string
	}{
		{BodySectionResponsibilities, "岗位职责", historical.Body.Responsibilities, latest.Body.Responsibilities},
		{BodySectionRequirements, "任职要求", historical.Body.Requirements, latest.Body.Requirements},
		{BodySectionSupplemental, "补充说明", bodySupplementalText(historical.Body.Supplemental), bodySupplementalText(latest.Body.Supplemental)},
	}
	for _, section := range sections {
		if normalizeComparisonText(section.history) == normalizeComparisonText(section.latest) {
			continue
		}
		comparison.BodySections = append(comparison.BodySections, ComparisonBodySection{
			Section: section.section, Label: section.label,
			Fragments: diffBodyFragments(section.history, section.latest),
		})
	}
	return comparison
}

func bodySupplementalText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatEducationRequirement(value *EducationRequirement) string {
	if value == nil || !value.ExplicitRequirement {
		return "-"
	}
	labels := map[EducationLevel]string{
		EducationLevelAssociateOrLower: "专科", EducationLevelBachelor: "本科",
		EducationLevelMaster: "硕士", EducationLevelDoctor: "博士",
	}
	if value.LowerBound != "" && value.UpperBound != "" {
		if value.LowerBound == value.UpperBound {
			return labels[value.LowerBound]
		}
		return labels[value.LowerBound] + "至" + labels[value.UpperBound]
	}
	if value.LowerBound != "" {
		if value.LowerBound == EducationLevelAssociateOrLower {
			return "专科及以上"
		}
		return labels[value.LowerBound] + "及以上"
	}
	if value.UpperBound != "" {
		if value.UpperBound == EducationLevelAssociateOrLower {
			return "专科及以下"
		}
		return labels[value.UpperBound] + "及以下"
	}
	return "-"
}

func formatExperienceRequirement(value *ExperienceRequirement) string {
	if value == nil || !value.ExplicitRequirement {
		return "-"
	}
	return "最低 " + formatInt(value.MinimumYears) + " 年"
}

func formatInt(value int) string {
	return strconv.Itoa(value)
}

func formatBoolean(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func formatFilterOption(value *FilterOption) string {
	if value == nil {
		return "-"
	}
	return displayOption(*value)
}

func formatFilterOptions(values []FilterOption) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, displayOption(value))
	}
	return formatStringSet(items)
}

func displayOption(value FilterOption) string {
	if value.Label != "" {
		return value.Label
	}
	return value.Value
}

func formatStringSet(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	unique := uniqueStrings(values)
	sort.Strings(unique)
	return strings.Join(unique, "、")
}

func equalStringSet(left, right []string) bool {
	return strings.Join(sortedUnique(left), "\x00") == strings.Join(sortedUnique(right), "\x00")
}

func equalFilterOption(left, right *FilterOption) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return displayOption(*left) == displayOption(*right)
}

func equalFilterOptions(left, right []FilterOption) bool {
	leftValues := make([]string, 0, len(left))
	rightValues := make([]string, 0, len(right))
	for _, value := range left {
		leftValues = append(leftValues, displayOption(value))
	}
	for _, value := range right {
		rightValues = append(rightValues, displayOption(value))
	}
	return equalStringSet(leftValues, rightValues)
}

func equalEducation(left, right *EducationRequirement) bool {
	return formatEducationRequirement(left) == formatEducationRequirement(right)
}

func equalExperience(left, right *ExperienceRequirement) bool {
	return formatExperienceRequirement(left) == formatExperienceRequirement(right)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func sortedUnique(values []string) []string {
	result := uniqueStrings(values)
	sort.Strings(result)
	return result
}

func normalizeComparisonText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return comparisonWhitespace.ReplaceAllString(value, " ")
}

func diffBodyFragments(historical, latest string) []ComparisonFragment {
	oldText, newText := normalizeComparisonText(historical), normalizeComparisonText(latest)
	if oldText == newText {
		return []ComparisonFragment{{Kind: ComparisonFragmentUnchanged, Text: historical}}
	}
	if oldText == "" {
		return []ComparisonFragment{{Kind: ComparisonFragmentInserted, Text: latest}}
	}
	if newText == "" {
		return []ComparisonFragment{{Kind: ComparisonFragmentDeleted, Text: historical}}
	}
	if reorderedBodyUnits(oldText, newText) {
		return []ComparisonFragment{{Kind: ComparisonFragmentDeleted, Text: historical}, {Kind: ComparisonFragmentInserted, Text: latest}}
	}
	return diffRunes(oldText, newText)
}

func reorderedBodyUnits(oldText, newText string) bool {
	oldUnits, newUnits := splitBodyUnits(oldText), splitBodyUnits(newText)
	if len(oldUnits) < 2 || len(oldUnits) != len(newUnits) {
		return false
	}
	oldIdentities := make([]string, 0, len(oldUnits))
	newIdentities := make([]string, 0, len(newUnits))
	for _, unit := range oldUnits {
		oldIdentities = append(oldIdentities, bodyUnitMarker.ReplaceAllString(unit, ""))
	}
	for _, unit := range newUnits {
		newIdentities = append(newIdentities, bodyUnitMarker.ReplaceAllString(unit, ""))
	}
	if equalStringSet(oldIdentities, newIdentities) {
		for index := range oldIdentities {
			if oldIdentities[index] != newIdentities[index] {
				return true
			}
		}
	}
	return false
}

func splitBodyUnits(value string) []string {
	lines := strings.Split(value, "\n")
	if len(lines) > 1 {
		return lines
	}
	runes := []rune(value)
	units := make([]string, 0, 2)
	start := 0
	for index, character := range runes {
		if !isSentenceTerminator(character) {
			continue
		}
		units = append(units, string(runes[start:index+1]))
		start = index + 1
	}
	if len(units) > 1 || start > 0 {
		if start < len(runes) {
			units = append(units, string(runes[start:]))
		}
		return units
	}
	return []string{value}
}

func isSentenceTerminator(character rune) bool {
	switch character {
	case '。', '；', ';', '！', '!', '？', '?':
		return true
	default:
		return false
	}
}

func diffRunes(oldText, newText string) []ComparisonFragment {
	oldRunes, newRunes := []rune(oldText), []rune(newText)
	type edit struct {
		kind ComparisonFragmentKind
		rune rune
	}
	max := len(oldRunes) + len(newRunes)
	v := map[int]int{1: 0}
	trace := make([]map[int]int, 0, max+1)
	endDistance := 0
	for distance := 0; distance <= max; distance++ {
		if distance > comparisonMaximumEditDistance {
			// A very large edit is still represented completely, but as two
			// blocks. This bounds Myers' trace memory for pathological JDs.
			return []ComparisonFragment{{Kind: ComparisonFragmentDeleted, Text: oldText}, {Kind: ComparisonFragmentInserted, Text: newText}}
		}
		trace = append(trace, cloneDiffFrontier(v))
		for diagonal := -distance; diagonal <= distance; diagonal += 2 {
			var x int
			if diagonal == -distance || (diagonal != distance && v[diagonal-1] < v[diagonal+1]) {
				x = v[diagonal+1]
			} else {
				x = v[diagonal-1] + 1
			}
			y := x - diagonal
			for x < len(oldRunes) && y < len(newRunes) && oldRunes[x] == newRunes[y] {
				x++
				y++
			}
			v[diagonal] = x
			if x >= len(oldRunes) && y >= len(newRunes) {
				endDistance = distance
				goto backtrack
			}
		}
	}

backtrack:
	edits := make([]edit, 0, max)
	x, y := len(oldRunes), len(newRunes)
	for distance := endDistance; distance > 0; distance-- {
		frontier := trace[distance]
		diagonal := x - y
		var previousDiagonal int
		if diagonal == -distance || (diagonal != distance && frontier[diagonal-1] < frontier[diagonal+1]) {
			previousDiagonal = diagonal + 1
		} else {
			previousDiagonal = diagonal - 1
		}
		previousX := frontier[previousDiagonal]
		previousY := previousX - previousDiagonal
		for x > previousX && y > previousY {
			edits = append(edits, edit{kind: ComparisonFragmentUnchanged, rune: oldRunes[x-1]})
			x--
			y--
		}
		if x == previousX {
			edits = append(edits, edit{kind: ComparisonFragmentInserted, rune: newRunes[y-1]})
			y--
		} else {
			edits = append(edits, edit{kind: ComparisonFragmentDeleted, rune: oldRunes[x-1]})
			x--
		}
	}
	for x > 0 && y > 0 {
		edits = append(edits, edit{kind: ComparisonFragmentUnchanged, rune: oldRunes[x-1]})
		x--
		y--
	}
	for x > 0 {
		edits = append(edits, edit{kind: ComparisonFragmentDeleted, rune: oldRunes[x-1]})
		x--
	}
	for y > 0 {
		edits = append(edits, edit{kind: ComparisonFragmentInserted, rune: newRunes[y-1]})
		y--
	}

	fragments := make([]ComparisonFragment, 0)
	for index := len(edits) - 1; index >= 0; index-- {
		current := edits[index]
		if len(fragments) > 0 && fragments[len(fragments)-1].Kind == current.kind {
			fragments[len(fragments)-1].Text += string(current.rune)
		} else {
			fragments = append(fragments, ComparisonFragment{Kind: current.kind, Text: string(current.rune)})
		}
	}
	return fragments
}

func cloneDiffFrontier(frontier map[int]int) map[int]int {
	clone := make(map[int]int, len(frontier))
	for diagonal, x := range frontier {
		clone[diagonal] = x
	}
	return clone
}
