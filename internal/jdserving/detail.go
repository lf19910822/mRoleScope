package jdserving

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"

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
       membership.body_content_version_id::text,
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
		&row.BodyContentVersionID,
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
	body := BodySections{Responsibilities: responsibilities, Requirements: requirements, Supplemental: supplemental}
	item, err := repository.loadListItem(ctx, row)
	if err != nil {
		return JobDetailResponse{}, err
	}
	highlights, err := repository.loadTechnologyHighlights(ctx, row.MembershipID, row.BodyContentVersionID, body)
	if err != nil {
		return JobDetailResponse{}, err
	}
	body.TechnologyHighlights = highlights
	return JobDetailResponse{
		ActivityState:       item.ActivityState,
		Body:                body,
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

type technologyHighlightCandidate struct {
	end         int
	sourceField string
	start       int
}

// loadTechnologyHighlights reads only the currently selected, same-body
// deterministic technology evidence. Earlier fallback resolutions remain
// valid for technology tags but cannot safely identify a substring in the
// current body, so they are intentionally excluded here.
func (repository *Repository) loadTechnologyHighlights(ctx context.Context, membershipID, bodyContentVersionID string, sections BodySections) ([]TechnologyHighlight, error) {
	rows, err := repository.pool.Query(ctx, `
SELECT evidence.source_field, evidence.codepoint_start, evidence.codepoint_end
FROM serving_jd_current_dimension_resolutions AS resolution
JOIN technology_analysis_results AS result ON result.id = resolution.technology_analysis_result_id
JOIN technology_mentions AS mention ON mention.analysis_result_id = result.id
JOIN technology_mention_evidence AS evidence ON evidence.technology_mention_id = mention.id
WHERE resolution.serving_jd_inventory_membership_id = $1
  AND resolution.dimension = 'technology'
  AND resolution.selection_priority = 0
  AND result.body_content_version_id = $2::uuid
ORDER BY evidence.source_field, evidence.codepoint_start, evidence.codepoint_end`, membershipID, bodyContentVersionID)
	if err != nil {
		return nil, fmt.Errorf("load serving technology highlights: %w", err)
	}
	defer rows.Close()

	candidates := make([]technologyHighlightCandidate, 0)
	for rows.Next() {
		var candidate technologyHighlightCandidate
		if err := rows.Scan(&candidate.sourceField, &candidate.start, &candidate.end); err != nil {
			return nil, fmt.Errorf("scan serving technology highlight: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate serving technology highlights: %w", err)
	}
	return sanitizeTechnologyHighlights(candidates, sections), nil
}

func sanitizeTechnologyHighlights(candidates []technologyHighlightCandidate, sections BodySections) []TechnologyHighlight {
	highlights := make([]TechnologyHighlight, 0, len(candidates))
	for _, candidate := range candidates {
		section, text, found := technologyHighlightSection(candidate.sourceField, sections)
		if !found || candidate.start < 0 || candidate.end <= candidate.start || candidate.end > utf8.RuneCountInString(text) {
			continue
		}
		highlights = append(highlights, TechnologyHighlight{
			End: candidate.end, Section: section, Start: candidate.start,
		})
	}

	sort.Slice(highlights, func(left, right int) bool {
		leftRank, rightRank := bodySectionRank(highlights[left].Section), bodySectionRank(highlights[right].Section)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if highlights[left].Start != highlights[right].Start {
			return highlights[left].Start < highlights[right].Start
		}
		return highlights[left].End < highlights[right].End
	})

	safe := make([]TechnologyHighlight, 0, len(highlights))
	for _, highlight := range highlights {
		previousIndex := len(safe) - 1
		if previousIndex >= 0 {
			previous := safe[previousIndex]
			if previous.Section == highlight.Section && highlight.Start < previous.End {
				continue
			}
		}
		safe = append(safe, highlight)
	}
	return safe
}

func technologyHighlightSection(sourceField string, sections BodySections) (BodySection, string, bool) {
	switch sourceField {
	case "description":
		return BodySectionResponsibilities, sections.Responsibilities, true
	case "jobRequire":
		return BodySectionRequirements, sections.Requirements, true
	case "addition":
		if sections.Supplemental == nil {
			return "", "", false
		}
		return BodySectionSupplemental, *sections.Supplemental, true
	default:
		return "", "", false
	}
}

func bodySectionRank(section BodySection) int {
	switch section {
	case BodySectionResponsibilities:
		return 0
	case BodySectionRequirements:
		return 1
	case BodySectionSupplemental:
		return 2
	default:
		return 3
	}
}
