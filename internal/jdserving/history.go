package jdserving

import (
	"sort"
	"strings"
	"time"
)

// historyObservation is the internal row-shaped fact used after the
// read-only PostgreSQL selection. It deliberately keeps provenance private;
// only the derived HistoryNode crosses the HTTP boundary.
type historyObservation struct {
	ActivityState           ActivityState
	BodyContentVersionID    string
	Body                    BodySections
	CanViewHistorical       bool
	Date                    string
	Detail                  JobDetailResponse
	Membership              listMembershipRow
	ParserFingerprint       string
	PresencePeriodID        string
	PublishedAt             time.Time
	RunID                   string
	SourceMetadataVersionID string
}

// selectFinalHistoryObservations is kept pure so the daily-final rule can be
// tested independently from PostgreSQL. The SQL reader may return one or
// more scope rows for a date; the latest successful publication wins.
func selectFinalHistoryObservations(rows []historyObservation) []historyObservation {
	ordered := append([]historyObservation(nil), rows...)
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].Date != ordered[right].Date {
			return ordered[left].Date < ordered[right].Date
		}
		if !ordered[left].PublishedAt.Equal(ordered[right].PublishedAt) {
			return ordered[left].PublishedAt.After(ordered[right].PublishedAt)
		}
		return ordered[left].RunID > ordered[right].RunID
	})
	selected := make([]historyObservation, 0, len(ordered))
	seenDate := make(map[string]struct{}, len(ordered))
	for _, row := range ordered {
		if row.Date == "" {
			continue
		}
		if _, exists := seenDate[row.Date]; exists {
			continue
		}
		seenDate[row.Date] = struct{}{}
		selected = append(selected, row)
	}
	sort.SliceStable(selected, func(left, right int) bool { return selected[left].Date < selected[right].Date })
	return selected
}

func buildHistoryNodes(observations []historyObservation) []HistoryNode {
	observations = selectFinalHistoryObservations(observations)
	if len(observations) == 0 {
		return []HistoryNode{}
	}
	latestIndex := len(observations) - 1
	nodes := make([]HistoryNode, 0, len(observations))
	lastEffective := observations[0]
	for index, current := range observations {
		var previous *historyObservation
		if index > 0 {
			previous = &observations[index-1]
		}
		isLatest := index == latestIndex
		var node *HistoryNode
		if index == 0 {
			node = newHistoryNode(current, HistoryNodeDiscovered, false)
			node.Tags = append(node.Tags, HistoryNodeTagDiscovered)
		} else {
			closedTransition := current.ActivityState == ActivityStateConfirmedClosed && previous.ActivityState != ActivityStateConfirmedClosed
			reopened := current.ActivityState == ActivityStateEffective && previous.ActivityState == ActivityStateConfirmedClosed && current.PresencePeriodID != previous.PresencePeriodID
			sourceChanged := historySourceChanged(*previous, current)
			switch {
			case reopened:
				node = newHistoryNode(current, HistoryNodeReopened, false)
				node.Tags = append(node.Tags, HistoryNodeTagReopened)
				if sourceChanged {
					appendHistoryComparisonTags(node, compareHistoryObservations(lastEffective, current))
				}
			case closedTransition:
				node = newHistoryNode(current, HistoryNodeConfirmedClosed, true)
				node.Tags = append(node.Tags, HistoryNodeTagConfirmedClosed)
			case sourceChanged:
				node = newHistoryNode(current, HistoryNodeModified, true)
				appendHistoryComparisonTags(node, compareHistoryObservations(*previous, current))
			case isLatest:
				node = newHistoryNode(current, HistoryNodeLatest, true)
				node.Tags = append(node.Tags, HistoryNodeTagLatest)
			}
		}
		if node != nil {
			if isLatest && !historyNodeHasTag(*node, HistoryNodeTagLatest) {
				node.Tags = append(node.Tags, HistoryNodeTagLatest)
			}
			nodes = append(nodes, *node)
		}
		if current.ActivityState == ActivityStateEffective {
			lastEffective = current
		}
	}
	return nodes
}

func newHistoryNode(observation historyObservation, nodeType HistoryNodeType, connectPrevious bool) *HistoryNode {
	return &HistoryNode{
		CanViewHistorical: observation.CanViewHistorical,
		ConnectPrevious:   connectPrevious,
		Date:              observation.Date,
		Summary:           historyNodeSummary(nodeType, nil),
		Tags:              []HistoryNodeTag{},
		Type:              nodeType,
	}
}

func historyNodeHasTag(node HistoryNode, tag HistoryNodeTag) bool {
	for _, candidate := range node.Tags {
		if candidate == tag {
			return true
		}
	}
	return false
}

func historySourceChanged(previous, current historyObservation) bool {
	return previous.SourceMetadataVersionID != current.SourceMetadataVersionID ||
		previous.BodyContentVersionID != current.BodyContentVersionID
}

func compareHistoryObservations(previous, current historyObservation) JobComparison {
	if !historySourceChanged(previous, current) {
		return JobComparison{Status: ComparisonStatusAvailable}
	}
	return compareJobDetails(previous.Detail, current.Detail)
}

func appendHistoryComparisonTags(node *HistoryNode, comparison JobComparison) {
	if len(comparison.DimensionChanges) > 0 {
		node.Tags = appendUniqueHistoryTag(node.Tags, HistoryNodeTagDimension)
	}
	for _, section := range comparison.BodySections {
		var tag HistoryNodeTag
		switch section.Section {
		case BodySectionResponsibilities:
			tag = HistoryNodeTagBodyResponsibilities
		case BodySectionRequirements:
			tag = HistoryNodeTagBodyRequirements
		case BodySectionSupplemental:
			tag = HistoryNodeTagBodySupplemental
		}
		if tag != "" {
			node.Tags = appendUniqueHistoryTag(node.Tags, tag)
		}
	}
	node.Summary = historyNodeSummary(node.Type, node.Tags)
}

func appendUniqueHistoryTag(tags []HistoryNodeTag, tag HistoryNodeTag) []HistoryNodeTag {
	for _, current := range tags {
		if current == tag {
			return tags
		}
	}
	return append(tags, tag)
}

func historyNodeSummary(nodeType HistoryNodeType, tags []HistoryNodeTag) string {
	parts := make([]string, 0, len(tags))
	for _, tag := range tags {
		switch tag {
		case HistoryNodeTagDimension:
			parts = append(parts, "维度变动")
		case HistoryNodeTagBodyResponsibilities:
			parts = append(parts, "岗位职责")
		case HistoryNodeTagBodyRequirements:
			parts = append(parts, "任职要求")
		case HistoryNodeTagBodySupplemental:
			parts = append(parts, "补充说明")
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "、")
	}
	switch nodeType {
	case HistoryNodeDiscovered:
		return "首次发现"
	case HistoryNodeConfirmedClosed:
		return "确认下线"
	case HistoryNodeReopened:
		return "重新上线"
	case HistoryNodeLatest:
		return "最新观察"
	default:
		return "岗位变动"
	}
}
