package aitrend

import "testing"

func TestBuildDraftReservesGroupsAndMergesDuplicateCurve(t *testing.T) {
	draft, err := BuildDraft([]CandidateGroup{
		{SubQueryID: "database", Candidates: []Candidate{{Dimension: DimensionTechnology, Value: "redis", Label: "Redis", MatchStatus: MatchExact, Score: 100}}},
		{SubQueryID: "cache", Candidates: []Candidate{{Dimension: DimensionTechnology, Value: "redis", Label: "Redis", MatchStatus: MatchExact, Score: 100}}},
	}, DraftBuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Curves) != 1 || len(draft.Curves[0].SubQueryIDs) != 2 {
		t.Fatalf("draft curves = %#v", draft.Curves)
	}
}

func TestBuildDraftPreservesPartialAndStopWarnings(t *testing.T) {
	draft, err := BuildDraft(nil, DraftBuildOptions{Warnings: []string{"部分工具失败"}, PartialFailure: true})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Status != DraftPartial || len(draft.Warnings) != 1 {
		t.Fatalf("draft = %#v", draft)
	}
	stopped, err := BuildDraft(nil, DraftBuildOptions{Stopped: true})
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Status != DraftStopped || len(stopped.Warnings) == 0 {
		t.Fatalf("stopped draft = %#v", stopped)
	}
}

func TestBuildDraftHonorsPerSubQueryRankingLimits(t *testing.T) {
	draft, err := BuildDraft([]CandidateGroup{
		{SubQueryID: "languages", Limit: 1, Candidates: []Candidate{
			{Dimension: DimensionTechnology, Value: "cpp", Label: "C++", MatchStatus: MatchExact, Score: 100},
			{Dimension: DimensionTechnology, Value: "python", Label: "Python", MatchStatus: MatchExact, Score: 90},
		}},
		{SubQueryID: "projects", Limit: 2, Candidates: []Candidate{
			{Dimension: DimensionProject, Value: "alpha", Label: "Alpha", MatchStatus: MatchExact, Score: 80},
			{Dimension: DimensionProject, Value: "beta", Label: "Beta", MatchStatus: MatchExact, Score: 70},
		}},
	}, DraftBuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Curves) != 3 {
		t.Fatalf("draft curves = %#v", draft.Curves)
	}
	for _, curve := range draft.Curves {
		if curve.Condition.Value == "python" {
			t.Fatalf("language limit was ignored: %#v", draft.Curves)
		}
	}
}
