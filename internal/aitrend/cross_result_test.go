package aitrend

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"mrolescope/internal/jdserving"
)

func TestCrossStatisticsPagerKeepsFullRankingIndependentOfChartLimit(t *testing.T) {
	result := jdserving.CrossStatisticsResult{Total: 200}
	for i := 0; i < 125; i++ {
		result.Groups = append(result.Groups, jdserving.CrossStatisticsGroup{Value: fmt.Sprintf("value-%03d", i), Label: fmt.Sprintf("value-%03d", i), Count: 125 - i})
	}
	for _, test := range []struct {
		name  string
		limit *int
		want  int
	}{
		{"default", nil, 8},
		{"more than charts", intPointer(20), 20},
		{"all including beyond model bound", intPointer(0), 125},
	} {
		t.Run(test.name, func(t *testing.T) {
			pager, err := NewCrossStatisticsPager(result, DimensionProject, test.limit, 7)
			if err != nil {
				t.Fatal(err)
			}
			var values []string
			for {
				page, err := pager.Next(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				if page.TotalJobs != 200 || page.TotalGroups != 125 || page.RequestedCount != test.want || page.Shown != len(values)+len(page.Groups) {
					t.Fatalf("invalid page state: %#v", page)
				}
				for _, group := range page.Groups {
					values = append(values, group.Value)
				}
				if !page.HasMore {
					if !page.Complete {
						t.Fatal("final page not marked complete for the requested scope")
					}
					break
				}
				if page.Complete {
					t.Fatal("unfinished pagination was marked complete")
				}
			}
			if len(values) != test.want {
				t.Fatalf("got %d values, want %d", len(values), test.want)
			}
			for index, value := range values {
				if value != fmt.Sprintf("value-%03d", index) {
					t.Fatalf("unstable page ordering at %d: %q", index, value)
				}
			}
		})
	}
}

func TestCrossStatisticsPagerCancellationDoesNotClaimComplete(t *testing.T) {
	result := jdserving.CrossStatisticsResult{Groups: []jdserving.CrossStatisticsGroup{{Value: "a"}, {Value: "b"}, {Value: "c"}}}
	pager, err := NewCrossStatisticsPager(result, DimensionLocation, intPointer(0), 1)
	if err != nil {
		t.Fatal(err)
	}
	first, err := pager.Next(context.Background())
	if err != nil || first.Shown != 1 || !first.HasMore {
		t.Fatalf("first page=%#v err=%v", first, err)
	}
	// A caller cannot mutate the already captured snapshot between pages.
	result.Groups[1].Value = "changed"
	second, err := pager.Next(context.Background())
	if err != nil || len(second.Groups) != 1 || second.Groups[0].Value != "b" {
		t.Fatalf("captured snapshot changed: page=%#v err=%v", second, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stopped, err := pager.Next(ctx)
	if !errors.Is(err, context.Canceled) || !stopped.Interrupted || stopped.Complete || stopped.Shown != 2 || stopped.RequestedCount != 3 {
		t.Fatalf("cancellation incorrectly claims completion: page=%#v err=%v", stopped, err)
	}
}

func TestCrossStatisticsPagerBudgetExhaustionMarksPartial(t *testing.T) {
	pager, err := NewCrossStatisticsPager(jdserving.CrossStatisticsResult{Groups: []jdserving.CrossStatisticsGroup{{Value: "a"}, {Value: "b"}}}, DimensionProject, intPointer(0), 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pager.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopped := pager.Interrupt()
	if stopped.Complete || !stopped.Interrupted || stopped.Shown != 1 || stopped.RequestedCount != 2 {
		t.Fatalf("budget exhaustion incorrectly claims completion: %#v", stopped)
	}
	if _, err := pager.Next(context.Background()); err == nil {
		t.Fatal("interrupted pager returned another page")
	}
}

func intPointer(value int) *int { return &value }
