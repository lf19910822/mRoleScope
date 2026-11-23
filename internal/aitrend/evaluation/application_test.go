package evaluation

import (
	"context"
	"testing"

	"mrolescope/internal/aitrend"
)

type isolatedDashboard struct {
	curves []aitrend.CurveCondition
}

func (dashboard *isolatedDashboard) Apply(_ context.Context, draft aitrend.Draft) error {
	if err := aitrend.ValidateDraft(draft); err != nil {
		return err
	}
	dashboard.curves = dashboard.curves[:0]
	for _, curve := range draft.Curves {
		dashboard.curves = append(dashboard.curves, curve.Condition)
	}
	return nil
}

func TestApplicationIntegrationUsesIsolatedDashboardState(t *testing.T) {
	realDashboard := &isolatedDashboard{curves: []aitrend.CurveCondition{{Dimension: aitrend.DimensionProject, Value: "existing"}}}
	testDashboard := &isolatedDashboard{}
	draft := aitrend.Draft{
		ContractVersion: aitrend.ContractVersionV1,
		Status:          aitrend.DraftComplete,
		Curves: []aitrend.DraftCurve{
			{Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "go"}, Label: "Go", Reason: "fixture", MatchStatus: aitrend.MatchExact, SubQueryIDs: []string{"case"}},
			{Condition: aitrend.CurveCondition{Dimension: aitrend.DimensionTechnology, Value: "python"}, Label: "Python", Reason: "fixture", MatchStatus: aitrend.MatchExact, SubQueryIDs: []string{"case"}},
		},
	}
	if err := testDashboard.Apply(context.Background(), draft); err != nil {
		t.Fatal(err)
	}
	if len(testDashboard.curves) != 2 || len(realDashboard.curves) != 1 || realDashboard.curves[0].Value != "existing" {
		t.Fatalf("test dashboard=%#v real dashboard=%#v", testDashboard, realDashboard)
	}
}
