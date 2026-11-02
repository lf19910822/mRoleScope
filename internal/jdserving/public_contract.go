package jdserving

// ActivityState is the viewer-facing lifecycle classification. A source-list
// absence still awaiting recheck is intentionally represented as effective.
type ActivityState string

const (
	ActivityStateConfirmedClosed ActivityState = "confirmed_closed"
	ActivityStateEffective       ActivityState = "effective"
)

// EducationLevel is a normalized education requirement level. The presence of
// a lower or upper bound explains its direction; it is not a processing state.
type EducationLevel string

const (
	EducationLevelAssociateOrLower EducationLevel = "associate_or_lower"
	EducationLevelBachelor         EducationLevel = "bachelor"
	EducationLevelDoctor           EducationLevel = "doctor"
	EducationLevelMaster           EducationLevel = "master"
)

// BodySection identifies one viewer-approved normalized JD body field. It is
// deliberately narrower than the source field names used in storage.
type BodySection string

const (
	BodySectionResponsibilities BodySection = "responsibilities"
	BodySectionRequirements     BodySection = "requirements"
	BodySectionSupplemental     BodySection = "supplemental"
)

// TechnologyHighlight is a display-safe Unicode code-point interval in one
// current returned body section. It does not carry a matched alias, rule,
// parser, result, source-body, or provenance identifier.
type TechnologyHighlight struct {
	End     int         `json:"end"`
	Section BodySection `json:"section"`
	Start   int         `json:"start"`
}

// BodySections are the viewer-approved, normalized JD body fields. A missing
// supplemental section is omitted rather than represented as an empty error.
type BodySections struct {
	Requirements         string                `json:"requirements"`
	Responsibilities     string                `json:"responsibilities"`
	Supplemental         *string               `json:"supplemental,omitempty"`
	TechnologyHighlights []TechnologyHighlight `json:"technology_highlights,omitempty"`
}

// EducationRequirement presents only the education meaning expressed by the
// deterministic result. An absent object means the field is not available;
// ExplicitRequirement false means the JD has no explicit education condition.
type EducationRequirement struct {
	ExplicitRequirement bool           `json:"explicit_requirement"`
	LowerBound          EducationLevel `json:"lower_bound,omitempty"`
	UpperBound          EducationLevel `json:"upper_bound,omitempty"`
}

// ExperienceRequirement presents the formal minimum work experience without
// exposing processing state or converting an absent requirement into zero.
type ExperienceRequirement struct {
	ExplicitRequirement bool `json:"explicit_requirement"`
	MinimumYears        int  `json:"minimum_years,omitempty"`
}

// FilterOption is a stable, display-safe selectable value. Its value is not a
// database key or source identifier.
type FilterOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// TechnologyKeywordFilterOption is a selectable formal technology mention
// enriched with its display-safe browse category. The category is vocabulary
// metadata, not a JD result, parser fact, or source attribute.
type TechnologyKeywordFilterOption struct {
	Category FilterOption `json:"category"`
	Label    string       `json:"label"`
	Value    string       `json:"value"`
}

// JobDetailResponse is the display-safe response for one opaque JD ID.
type JobDetailResponse struct {
	ActivityState       ActivityState          `json:"activity_state"`
	Body                BodySections           `json:"body"`
	Education           *EducationRequirement  `json:"education,omitempty"`
	Experience          *ExperienceRequirement `json:"experience,omitempty"`
	FirstDiscoveredDate string                 `json:"first_discovered_date"`
	ID                  string                 `json:"id"`
	IsUrgent            bool                   `json:"is_urgent"`
	Locations           []string               `json:"locations"`
	MainJobFamily       *FilterOption          `json:"main_job_family,omitempty"`
	OtherJobFamilies    []FilterOption         `json:"other_job_families"`
	Projects            []FilterOption         `json:"projects"`
	TechnologyKeywords  []FilterOption         `json:"technology_keywords"`
	Title               string                 `json:"title"`
}

// JobListItem is the display-safe, body-free representation used in a paged
// list. The ID remains opaque to the caller.
type JobListItem struct {
	ActivityState       ActivityState          `json:"activity_state"`
	Education           *EducationRequirement  `json:"education,omitempty"`
	Experience          *ExperienceRequirement `json:"experience,omitempty"`
	FirstDiscoveredDate string                 `json:"first_discovered_date"`
	ID                  string                 `json:"id"`
	IsUrgent            bool                   `json:"is_urgent"`
	Locations           []string               `json:"locations"`
	MainJobFamily       *FilterOption          `json:"main_job_family,omitempty"`
	OtherJobFamilies    []FilterOption         `json:"other_job_families"`
	Projects            []FilterOption         `json:"projects"`
	TechnologyKeywords  []FilterOption         `json:"technology_keywords"`
	Title               string                 `json:"title"`
}

// JobListResponse is the typed paginated list contract.
type JobListResponse struct {
	Items []JobListItem `json:"items"`
	Page  PageInfo      `json:"page"`
}

// FilterOptionsResponse contains the approved structured filter vocabulary.
// First-discovered date is selected through a client date control, rather
// than an unbounded list of individual dates.
type FilterOptionsResponse struct {
	ActivityStates     []FilterOption                  `json:"activity_states"`
	EducationLevels    []FilterOption                  `json:"education_levels"`
	ExperienceYears    []FilterOption                  `json:"experience_years"`
	JobFamilies        []FilterOption                  `json:"job_families"`
	Locations          []FilterOption                  `json:"locations"`
	MainJobFamilies    []FilterOption                  `json:"main_job_families"`
	OtherJobFamilies   []FilterOption                  `json:"other_job_families"`
	Projects           []FilterOption                  `json:"projects"`
	TechnologyKeywords []TechnologyKeywordFilterOption `json:"technology_keywords"`
	UrgentOptions      []FilterOption                  `json:"urgent_options"`
}

// PageInfo contains the bounded, display-safe pagination state.
type PageInfo struct {
	Number   int `json:"number"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}
