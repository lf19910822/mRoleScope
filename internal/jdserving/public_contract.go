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

// ComparisonStatus describes whether a historical detail comparison was
// computed. It is intentionally independent from the job lifecycle state.
type ComparisonStatus string

const (
	ComparisonStatusAvailable   ComparisonStatus = "available"
	ComparisonStatusUnavailable ComparisonStatus = "unavailable"
)

type ComparisonFragmentKind string

const (
	ComparisonFragmentUnchanged ComparisonFragmentKind = "unchanged"
	ComparisonFragmentDeleted   ComparisonFragmentKind = "deleted"
	ComparisonFragmentInserted  ComparisonFragmentKind = "inserted"
)

// ComparisonFragment is an ordered, display-safe body diff fragment.
type ComparisonFragment struct {
	Kind ComparisonFragmentKind `json:"kind"`
	Text string                 `json:"text"`
}

// ComparisonDimensionChange contains only the formatted values a reader can
// verify. Field is a stable display label, not a storage column name.
type ComparisonDimensionChange struct {
	Field      string `json:"field"`
	Historical string `json:"historical"`
	Latest     string `json:"latest"`
}

// ComparisonBodySection contains the complete ordered diff for one changed
// body section. Empty/equal sections are omitted from a successful result.
type ComparisonBodySection struct {
	Fragments []ComparisonFragment `json:"fragments"`
	Label     string               `json:"label"`
	Section   BodySection          `json:"section"`
}

// JobComparison is optional on JobDetailResponse. Unavailable comparisons
// retain the historical JD and expose only a bounded user-facing message.
type JobComparison struct {
	BodySections     []ComparisonBodySection     `json:"body_sections,omitempty"`
	DimensionChanges []ComparisonDimensionChange `json:"dimension_changes,omitempty"`
	Message          string                      `json:"message,omitempty"`
	Retryable        bool                        `json:"retryable,omitempty"`
	Status           ComparisonStatus            `json:"status"`
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
	Comparison          *JobComparison         `json:"comparison,omitempty"`
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
	HistoryDate         string                 `json:"history_date,omitempty"`
	HistoryNodeType     HistoryNodeType        `json:"history_node_type,omitempty"`
}

// HistoryNodeType is the stable, display-safe event classification for one
// successful Shanghai business-day observation.
type HistoryNodeType string

const (
	HistoryNodeDiscovered      HistoryNodeType = "discovered"
	HistoryNodeModified        HistoryNodeType = "modified"
	HistoryNodeConfirmedClosed HistoryNodeType = "confirmed_closed"
	HistoryNodeReopened        HistoryNodeType = "reopened"
	HistoryNodeLatest          HistoryNodeType = "latest"
)

// HistoryNodeTag is a compact machine-stable label. The UI maps it to the
// approved Chinese copy; no source, parser, or database identity crosses the
// public boundary.
type HistoryNodeTag string

const (
	HistoryNodeTagDiscovered           HistoryNodeTag = "discovered"
	HistoryNodeTagLatest               HistoryNodeTag = "latest"
	HistoryNodeTagDimension            HistoryNodeTag = "dimension"
	HistoryNodeTagBodyResponsibilities HistoryNodeTag = "body_responsibilities"
	HistoryNodeTagBodyRequirements     HistoryNodeTag = "body_requirements"
	HistoryNodeTagBodySupplemental     HistoryNodeTag = "body_supplemental"
	HistoryNodeTagConfirmedClosed      HistoryNodeTag = "confirmed_closed"
	HistoryNodeTagReopened             HistoryNodeTag = "reopened"
)

// HistoryNode contains only what the timeline needs to render and route a
// node. ConnectPrevious is false for the first node and for a reopen after a
// confirmed close, so the client never draws a misleading lifecycle bridge.
type HistoryNode struct {
	CanViewHistorical bool             `json:"can_view_historical"`
	ConnectPrevious   bool             `json:"connect_previous"`
	Date              string           `json:"date"`
	Summary           string           `json:"summary"`
	Tags              []HistoryNodeTag `json:"tags"`
	Type              HistoryNodeType  `json:"type"`
}

// JobHistoryResponse is the display-safe history summary for one opaque job.
type JobHistoryResponse struct {
	Nodes []HistoryNode `json:"nodes"`
}

// JobListItem is the display-safe, body-free representation used in a paged
// list. The ID remains opaque to the caller.
type JobListItem struct {
	ActivityState       ActivityState          `json:"activity_state"`
	Education           *EducationRequirement  `json:"education,omitempty"`
	Experience          *ExperienceRequirement `json:"experience,omitempty"`
	FirstDiscoveredDate string                 `json:"first_discovered_date"`
	LastModifiedDate    *string                `json:"last_modified_date"`
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

// TrendDrilldownStatus is the current availability of a job that belonged to
// a historical trend node. It deliberately describes only the reader's next
// action, never the source or SCD2 reason behind it.
type TrendDrilldownStatus string

const (
	TrendDrilldownStatusChanged TrendDrilldownStatus = "changed"
	TrendDrilldownStatusClosed  TrendDrilldownStatus = "confirmed_closed"
	TrendDrilldownStatusCurrent TrendDrilldownStatus = "current"
)

// TrendDrilldownItem is a compact historical-node list item. ID is opaque
// transport identity for opening a detail and is not a user-facing field.
type TrendDrilldownItem struct {
	CanViewHistorical bool                 `json:"can_view_historical"`
	CanViewLatest     bool                 `json:"can_view_latest"`
	ID                string               `json:"id"`
	Status            TrendDrilldownStatus `json:"status"`
	Title             string               `json:"title"`
}

// TrendDrilldownResponse describes one historical curve node. The node date
// is a user-facing business date; all run/version provenance remains hidden.
type TrendDrilldownResponse struct {
	// CountMatchesExpected is present only for an unsearched node request. It
	// distinguishes a changed historical collection from a title filter that
	// intentionally makes MatchedTotal smaller than the chart point.
	CountMatchesExpected *bool                `json:"count_matches_expected,omitempty"`
	Items                []TrendDrilldownItem `json:"items"`
	MatchedTotal         int                  `json:"matched_total"`
	NodeDate             string               `json:"node_date"`
	Page                 PageInfo             `json:"page"`
}
