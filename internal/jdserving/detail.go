package jdserving

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrJobNotFound is returned only when an opaque public ID is not part of the
// current Serving inventory. It carries no source or provenance detail.
var ErrJobNotFound = errors.New("serving JD not found")

// GetJobDetail returns one current Serving JD together with the normalized
// body sections anchored by its immutable current inventory membership. It
// does not query historical text, audit state, parser metadata, or AI output.
func (repository *Repository) GetJobDetail(ctx context.Context, publicID string) (JobDetailResponse, error) {
	if repository == nil || repository.pool == nil {
		return JobDetailResponse{}, fmt.Errorf("serving repository is unavailable")
	}
	var row listMembershipRow
	var responsibilities, requirements string
	var supplemental *string
	err := repository.pool.QueryRow(ctx, `
SELECT membership.id::text,
       identifier.public_id::text,
       membership.title,
       membership.locations,
       membership.activity_state::text,
       membership.first_discovered_at::date::text,
       membership.is_urgent,
       body.description_text,
       body.job_require_text,
       NULLIF(body.addition_text, '')
FROM serving_jd_current_inventory_memberships AS membership
JOIN serving_jd_public_identifiers AS identifier ON identifier.job_id = membership.job_id
JOIN job_body_content_versions AS body ON body.id = membership.body_content_version_id
WHERE identifier.public_id::text = $1`, publicID).Scan(
		&row.MembershipID,
		&row.ID,
		&row.Title,
		&row.Locations,
		&row.ActivityState,
		&row.FirstDiscoveredDate,
		&row.IsUrgent,
		&responsibilities,
		&requirements,
		&supplemental,
	)
	if err == pgx.ErrNoRows {
		return JobDetailResponse{}, ErrJobNotFound
	}
	if err != nil {
		return JobDetailResponse{}, fmt.Errorf("load serving JD detail: %w", err)
	}
	item, err := repository.loadListItem(ctx, row)
	if err != nil {
		return JobDetailResponse{}, err
	}
	return JobDetailResponse{
		ActivityState:       item.ActivityState,
		Body:                BodySections{Responsibilities: responsibilities, Requirements: requirements, Supplemental: supplemental},
		Education:           item.Education,
		Experience:          item.Experience,
		FirstDiscoveredDate: item.FirstDiscoveredDate,
		ID:                  item.ID,
		IsUrgent:            item.IsUrgent,
		Locations:           item.Locations,
		MainJobFamily:       item.MainJobFamily,
		OtherJobFamilies:    item.OtherJobFamilies,
		Projects:            item.Projects,
		TechnologyKeywords:  item.TechnologyKeywords,
		Title:               item.Title,
	}, nil
}
