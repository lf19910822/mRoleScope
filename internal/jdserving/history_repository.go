package jdserving

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// GetJobHistory reads one final successful publication per Shanghai business
// day. It intentionally does not persist or reconstruct an event table.
func (repository *Repository) GetJobHistory(ctx context.Context, publicID string) (JobHistoryResponse, error) {
	if repository == nil || repository.pool == nil {
		return JobHistoryResponse{}, fmt.Errorf("serving repository is unavailable")
	}
	if publicID == "" {
		return JobHistoryResponse{}, ErrJobNotFound
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return JobHistoryResponse{}, fmt.Errorf("begin history read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := repository.loadHistoryObservations(ctx, tx, publicID)
	if err != nil {
		return JobHistoryResponse{}, err
	}
	if rows == nil {
		return JobHistoryResponse{}, ErrJobNotFound
	}
	for index := range rows {
		rows[index].CanViewHistorical = true
		if detail, detailErr := repository.loadHistorySnapshot(ctx, tx, rows[index]); detailErr == nil {
			rows[index].Detail = detail
		} else {
			rows[index].CanViewHistorical = false
		}
	}
	response := JobHistoryResponse{Nodes: buildHistoryNodes(rows)}
	if err := tx.Commit(ctx); err != nil {
		return JobHistoryResponse{}, fmt.Errorf("commit history read: %w", err)
	}
	return response, nil
}

// GetJobHistoryDetail returns the snapshot represented by one timeline node.
// Modified and reopened nodes compare against the nearest earlier successful
// source observation; single-version nodes intentionally have no comparison.
func (repository *Repository) GetJobHistoryDetail(ctx context.Context, publicID string, nodeDate time.Time, nodeType HistoryNodeType) (JobDetailResponse, error) {
	if repository == nil || repository.pool == nil {
		return JobDetailResponse{}, fmt.Errorf("serving repository is unavailable")
	}
	if err := validateHistoricalDetailDate(nodeDate); err != nil {
		return JobDetailResponse{}, err
	}
	if !validHistoryNodeType(nodeType) {
		return JobDetailResponse{}, ErrInvalidTrendQuery
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return JobDetailResponse{}, fmt.Errorf("begin history detail read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := repository.loadHistoryObservations(ctx, tx, publicID)
	if err != nil {
		return JobDetailResponse{}, err
	}
	if rows == nil {
		return JobDetailResponse{}, ErrJobNotFound
	}
	date := nodeDate.In(mustShanghai()).Format("2006-01-02")
	selectedIndex := -1
	for index := range rows {
		if rows[index].Date == date {
			selectedIndex = index
			break
		}
	}
	if selectedIndex < 0 {
		return JobDetailResponse{}, ErrHistoricalDetailUnavailable
	}
	selected := rows[selectedIndex]
	knownNodes := buildHistoryNodes(rows)
	knownType := HistoryNodeType("")
	for _, node := range knownNodes {
		if node.Date == selected.Date {
			knownType = node.Type
			break
		}
	}
	if knownType == "" || knownType != nodeType {
		return JobDetailResponse{}, ErrHistoricalDetailUnavailable
	}
	selectedDetail, err := repository.loadHistorySnapshot(ctx, tx, selected)
	if err != nil {
		return JobDetailResponse{}, ErrHistoricalDetailUnavailable
	}
	var comparison *JobComparison
	if nodeType == HistoryNodeModified || nodeType == HistoryNodeReopened {
		baselineIndex := historyBaselineIndex(rows, selectedIndex, nodeType)
		if baselineIndex >= 0 && historySourceChanged(rows[baselineIndex], selected) {
			baselineDetail, baselineErr := repository.loadHistorySnapshot(ctx, tx, rows[baselineIndex])
			if baselineErr != nil {
				return JobDetailResponse{}, ErrHistoricalDetailUnavailable
			}
			computed := compareJobDetails(baselineDetail, selectedDetail)
			comparison = &computed
		}
	}
	selectedDetail.Comparison = comparison
	selectedDetail.HistoryDate = selected.Date
	selectedDetail.HistoryNodeType = nodeType
	if err := tx.Commit(ctx); err != nil {
		return JobDetailResponse{}, fmt.Errorf("commit history detail read: %w", err)
	}
	return selectedDetail, nil
}

func validHistoryNodeType(nodeType HistoryNodeType) bool {
	switch nodeType {
	case HistoryNodeDiscovered, HistoryNodeModified, HistoryNodeConfirmedClosed, HistoryNodeReopened, HistoryNodeLatest:
		return true
	default:
		return false
	}
}

func historyBaselineIndex(rows []historyObservation, selectedIndex int, nodeType HistoryNodeType) int {
	if selectedIndex <= 0 {
		return -1
	}
	if nodeType == HistoryNodeReopened {
		for index := selectedIndex - 1; index >= 0; index-- {
			if rows[index].ActivityState == ActivityStateEffective {
				return index
			}
		}
		return -1
	}
	return selectedIndex - 1
}

type historyObservationScan struct {
	MembershipID            string
	PublicID                string
	Title                   string
	Locations               []string
	ActivityState           string
	FirstDiscoveredDate     string
	IsUrgent                bool
	SourceMetadataVersionID string
	BodyContentVersionID    string
	Responsibilities        string
	Requirements            string
	Supplemental            *string
	RunID                   string
	PublishedAt             time.Time
	Date                    string
	PresencePeriodID        string
}

func (repository *Repository) loadHistoryObservations(ctx context.Context, executor queryExecutor, publicID string) ([]historyObservation, error) {
	rows, err := executor.Query(ctx, `
WITH scope_candidates AS (
    SELECT membership.observation_scope_id,
           run.published_at,
           run.id AS run_id,
           0 AS priority
    FROM serving_jd_current_inventory_memberships AS membership
    JOIN collection_runs AS run ON run.id = membership.published_run_id
    JOIN serving_jd_public_identifiers AS identifier ON identifier.job_id = membership.job_id
    WHERE identifier.public_id::text = $1
    UNION ALL
    SELECT membership.observation_scope_id,
           run.published_at,
           run.id AS run_id,
           1 AS priority
    FROM serving_jd_inventory_memberships AS membership
    JOIN collection_runs AS run ON run.id = membership.published_run_id
    JOIN serving_jd_public_identifiers AS identifier ON identifier.job_id = membership.job_id
    WHERE identifier.public_id::text = $1
), selected_scope AS (
    SELECT observation_scope_id
    FROM scope_candidates
    ORDER BY priority, published_at DESC, run_id DESC
    LIMIT 1
), candidates AS (
    SELECT membership.id::text AS membership_id,
           identifier.public_id::text AS public_id,
           membership.title,
           membership.locations,
           membership.activity_state::text,
           membership.first_discovered_at::date::text,
           membership.is_urgent,
           membership.source_metadata_version_id::text,
           membership.body_content_version_id::text,
           body.description_text,
           body.job_require_text,
           NULLIF(body.addition_text, '') AS addition_text,
           run.id::text AS run_id,
           run.published_at,
           (run.published_at AT TIME ZONE 'Asia/Shanghai')::date::text AS calendar_date,
           membership.presence_period_id::text
    FROM serving_jd_inventory_memberships AS membership
    JOIN collection_runs AS run ON run.id = membership.published_run_id
    JOIN serving_jd_public_identifiers AS identifier ON identifier.job_id = membership.job_id
    JOIN job_body_content_versions AS body ON body.id = membership.body_content_version_id
    JOIN selected_scope AS scope ON scope.observation_scope_id = membership.observation_scope_id
    WHERE identifier.public_id::text = $1
      AND run.status = 'published'
      AND run.published_at IS NOT NULL
), scope_daily AS (
    SELECT DISTINCT ON (calendar_date, membership_id) candidates.*
    FROM candidates
    ORDER BY calendar_date, membership_id, published_at DESC, run_id DESC
), daily AS (
    SELECT DISTINCT ON (calendar_date) scope_daily.*
    FROM scope_daily
    ORDER BY calendar_date, published_at DESC, run_id DESC
)
SELECT membership_id, public_id, title, locations, activity_state,
       first_discovered_at, is_urgent, source_metadata_version_id,
       body_content_version_id, description_text, job_require_text,
       addition_text, run_id, published_at, calendar_date, presence_period_id
FROM daily
ORDER BY calendar_date`, publicID)
	if err != nil {
		return nil, fmt.Errorf("select JD history observations: %w", err)
	}
	defer rows.Close()
	observations := make([]historyObservation, 0)
	for rows.Next() {
		var scan historyObservationScan
		if err := rows.Scan(
			&scan.MembershipID, &scan.PublicID, &scan.Title, &scan.Locations,
			&scan.ActivityState, &scan.FirstDiscoveredDate, &scan.IsUrgent,
			&scan.SourceMetadataVersionID, &scan.BodyContentVersionID,
			&scan.Responsibilities, &scan.Requirements, &scan.Supplemental,
			&scan.RunID, &scan.PublishedAt, &scan.Date, &scan.PresencePeriodID,
		); err != nil {
			return nil, fmt.Errorf("scan JD history observation: %w", err)
		}
		observations = append(observations, historyObservation{
			ActivityState:           ActivityState(scan.ActivityState),
			BodyContentVersionID:    scan.BodyContentVersionID,
			Body:                    BodySections{Responsibilities: scan.Responsibilities, Requirements: scan.Requirements, Supplemental: scan.Supplemental},
			CanViewHistorical:       true,
			Date:                    scan.Date,
			Membership:              listMembershipRow{ActivityState: scan.ActivityState, BodyContentVersionID: scan.BodyContentVersionID, FirstDiscoveredDate: scan.FirstDiscoveredDate, ID: scan.PublicID, IsUrgent: scan.IsUrgent, Locations: scan.Locations, MembershipID: scan.MembershipID, SourceMetadataVersionID: scan.SourceMetadataVersionID, Title: scan.Title},
			PresencePeriodID:        scan.PresencePeriodID,
			PublishedAt:             scan.PublishedAt,
			RunID:                   scan.RunID,
			SourceMetadataVersionID: scan.SourceMetadataVersionID,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate JD history observations: %w", err)
	}
	if len(observations) == 0 {
		var exists bool
		if err := executor.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM serving_jd_public_identifiers WHERE public_id::text = $1)`, publicID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("check JD history public identifier: %w", err)
		}
		if !exists {
			return nil, nil
		}
	}
	return observations, nil
}

func (repository *Repository) loadHistorySnapshot(ctx context.Context, executor queryExecutor, observation historyObservation) (JobDetailResponse, error) {
	item, err := repository.loadListItemFrom(ctx, executor, observation.Membership)
	if err != nil {
		return JobDetailResponse{}, err
	}
	body := observation.Body
	highlights, err := repository.loadTechnologyHighlightsFrom(ctx, executor, observation.Membership.MembershipID, observation.Membership.BodyContentVersionID, body)
	if err != nil {
		return JobDetailResponse{}, err
	}
	body.TechnologyHighlights = highlights
	return detailResponse(item, body, nil), nil
}

func mustShanghai() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return location
}
