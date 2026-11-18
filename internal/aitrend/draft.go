package aitrend

import "fmt"

type CandidateGroup struct {
	SubQueryID string
	Candidates []Candidate
}

type DraftBuildOptions struct {
	Warnings       []string
	Explanation    string
	Validate       func(Draft) error
	Stopped        bool
	PartialFailure bool
}

// BuildDraft is the final deterministic step after tool reads. It reserves a
// row for each valid sub-question, merges duplicate identities, keeps all
// selected rows checked by default, and revalidates against the active
// publication before returning.
func BuildDraft(groups []CandidateGroup, options DraftBuildOptions) (Draft, error) {
	groupCandidates := make([][]Candidate, 0, len(groups))
	all := make([]Candidate, 0)
	for _, group := range groups {
		for _, candidate := range group.Candidates {
			if candidate.SubQueryID == "" {
				candidate.SubQueryID = group.SubQueryID
			}
			all = append(all, candidate)
		}
	}
	all = MergeCandidates(all)
	for _, group := range groups {
		candidates := make([]Candidate, 0, len(group.Candidates))
		for _, candidate := range all {
			belongs := candidate.SubQueryID == group.SubQueryID
			for _, id := range candidate.SubQueryIDs {
				if id == group.SubQueryID {
					belongs = true
					break
				}
			}
			if belongs {
				candidates = append(candidates, candidate)
			}
		}
		groupCandidates = append(groupCandidates, candidates)
	}
	curves := AllocateCandidates(groupCandidates)
	draft := Draft{ContractVersion: ContractVersionV1, Status: DraftComplete, Curves: make([]DraftCurve, 0, len(curves)), Warnings: append([]string(nil), options.Warnings...), Explanation: options.Explanation}
	if options.Stopped {
		draft.Status = DraftStopped
	}
	if options.PartialFailure && !options.Stopped {
		draft.Status = DraftPartial
	}
	for _, candidate := range curves {
		status := candidate.MatchStatus
		if status == "" {
			status = MatchPossible
		}
		label := candidate.Label
		if label == "" {
			label = candidate.Value
		}
		subQueryIDs := append([]string(nil), candidate.SubQueryIDs...)
		if len(subQueryIDs) == 0 && candidate.SubQueryID != "" {
			subQueryIDs = []string{candidate.SubQueryID}
		}
		if len(subQueryIDs) == 0 {
			return Draft{}, fmt.Errorf("candidate %q has no sub-query", candidate.Value)
		}
		reason := "根据当前正式值匹配"
		switch status {
		case MatchAlias:
			reason = "按正式别名匹配"
		case MatchPossible:
			reason = "可能匹配，保留供确认"
		case MatchExpansion:
			reason = "扩展推荐，先展示结果"
		}
		draft.Curves = append(draft.Curves, DraftCurve{
			Condition: CurveCondition{Dimension: candidate.Dimension, Value: candidate.Value, Operator: candidate.Operator, IncludeSecondary: candidate.IncludeSecondary},
			Label:     label, Selected: true, MatchStatus: status, Scope: candidate.Scope, Reason: reason, SubQueryIDs: subQueryIDs,
		})
	}
	if len(draft.Curves) == 0 && len(draft.Warnings) == 0 {
		draft.Warnings = append(draft.Warnings, "没有找到可用的正式值；可尝试更具体的技术名、项目名或岗位族名。")
	}
	if options.Validate != nil {
		if err := options.Validate(draft); err != nil {
			return Draft{}, err
		}
	} else if err := ValidateDraft(draft); err != nil {
		return Draft{}, err
	}
	return draft, nil
}
