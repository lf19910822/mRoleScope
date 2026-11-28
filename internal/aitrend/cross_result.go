package aitrend

import (
	"context"
	"fmt"

	"mrolescope/internal/jdserving"
)

// CrossStatisticsPage is safe to expose to the model and client. Page values
// are copied from the same complete repository result; none contains job IDs,
// descriptions, SQL, or internal publication identities.
type CrossStatisticsPage struct {
	TotalJobs      int                              `json:"total_jobs"`
	GroupBy        Dimension                        `json:"group_by,omitempty"`
	Groups         []jdserving.CrossStatisticsGroup `json:"groups,omitempty"`
	TotalGroups    int                              `json:"total_groups"`
	RequestedCount int                              `json:"requested_count"`
	Offset         int                              `json:"offset"`
	Shown          int                              `json:"shown"`
	HasMore        bool                             `json:"has_more"`
	Complete       bool                             `json:"complete"`
	Interrupted    bool                             `json:"interrupted,omitempty"`
}

// CrossStatisticsPager retains one immutable aggregate snapshot. A zero
// ranking limit requests all groups; nil requests the product default eight.
type CrossStatisticsPager struct {
	result    jdserving.CrossStatisticsResult
	groupBy   Dimension
	pageSize  int
	requested int
	offset    int
	stopped   bool
}

func NewCrossStatisticsPager(result jdserving.CrossStatisticsResult, groupBy Dimension, rankingLimit *int, pageSize int) (*CrossStatisticsPager, error) {
	if groupBy != "" && !isCrossDimension(groupBy) {
		return nil, fmt.Errorf("unsupported cross-statistics grouping")
	}
	if groupBy == "" && (len(result.Groups) > 0 || rankingLimit != nil) {
		return nil, fmt.Errorf("ranking requires a grouping dimension")
	}
	if pageSize == 0 {
		pageSize = MaxCrossPageSize
	}
	if pageSize < 1 || pageSize > MaxCrossPageSize || rankingLimit != nil && (*rankingLimit < 0 || *rankingLimit > 1_000_000) {
		return nil, fmt.Errorf("invalid cross-statistics page bounds")
	}
	result.Groups = append([]jdserving.CrossStatisticsGroup(nil), result.Groups...)
	requested := len(result.Groups)
	if groupBy != "" && rankingLimit == nil && requested > DefaultRankingLimit {
		requested = DefaultRankingLimit
	} else if rankingLimit != nil && *rankingLimit > 0 && requested > *rankingLimit {
		requested = *rankingLimit
	}
	return &CrossStatisticsPager{result: result, groupBy: groupBy, pageSize: pageSize, requested: requested}, nil
}

// Next returns a page or, on cancellation, an explicit interrupted state
// showing how much of the requested ranking was delivered so far.
func (pager *CrossStatisticsPager) Next(ctx context.Context) (CrossStatisticsPage, error) {
	if pager == nil {
		return CrossStatisticsPage{}, fmt.Errorf("cross-statistics pager is unavailable")
	}
	page := CrossStatisticsPage{
		TotalJobs: pager.result.Total, GroupBy: pager.groupBy,
		TotalGroups: len(pager.result.Groups), RequestedCount: pager.requested,
		Offset: pager.offset, Shown: pager.offset,
	}
	if err := ctx.Err(); err != nil {
		return pager.Interrupt(), err
	}
	if pager.stopped {
		page.Interrupted = true
		return page, fmt.Errorf("cross-statistics pagination was interrupted")
	}
	end := pager.offset + pager.pageSize
	if end > pager.requested {
		end = pager.requested
	}
	page.Groups = append([]jdserving.CrossStatisticsGroup(nil), pager.result.Groups[pager.offset:end]...)
	pager.offset = end
	page.Shown = end
	page.HasMore = end < pager.requested
	page.Complete = !page.HasMore
	return page, nil
}

// Interrupt records a caller budget exhaustion as a partial result with the
// exact number of already delivered groups. It does not fabricate a final page.
func (pager *CrossStatisticsPager) Interrupt() CrossStatisticsPage {
	if pager == nil {
		return CrossStatisticsPage{Interrupted: true}
	}
	pager.stopped = true
	return CrossStatisticsPage{
		TotalJobs: pager.result.Total, GroupBy: pager.groupBy,
		TotalGroups: len(pager.result.Groups), RequestedCount: pager.requested,
		Offset: pager.offset, Shown: pager.offset, Interrupted: true,
	}
}
