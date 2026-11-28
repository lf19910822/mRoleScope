package jobsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"mrolescope/internal/aitrend"
	"mrolescope/internal/jdserving"
)

type crossFixtureProvider struct{ turns []aitrend.IntentToolTurn }

func (provider *crossFixtureProvider) AnalyzeIntent(context.Context, string, *aitrend.ContractError) ([]byte, error) {
	return nil, errors.New("fixture uses tool turns")
}

func (provider *crossFixtureProvider) AnalyzeIntentTurn(context.Context, string, *aitrend.ContractError, []aitrend.ToolConversation, bool) (aitrend.IntentToolTurn, error) {
	if len(provider.turns) == 0 {
		return aitrend.IntentToolTurn{}, errors.New("fixture has no remaining model turns")
	}
	turn := provider.turns[0]
	provider.turns = provider.turns[1:]
	return turn, nil
}

type crossFixtureTrendReader struct{}

func (crossFixtureTrendReader) ReadTrendSummary(_ context.Context, query jdserving.TrendQuery) (jdserving.TrendSummary, error) {
	count := 1
	return jdserving.TrendSummary{Curve: jdserving.TrendCurve{Dimension: query.Condition.Dimension, Value: query.Condition.Value, Label: query.Condition.Value}, PeriodEndActiveJobs: &count, ValidObservationDays: 1}, nil
}

func TestCrossStatisticsCurrentFiltersAndGroups(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC)
	fixtures := []struct {
		id, location, family, secondary, project string
		technologies                             []string
	}{
		{"a", "上海", "backend", "operations", "project-a", []string{"java", "python"}},
		{"b", "上海", "frontend", "operations", "project-a", []string{"java"}},
		{"c", "北京", "backend", "operations", "project-b", []string{"python"}},
		{"d", "上海", "backend", "operations", "project-b", nil},
	}
	observations := make([]Observation, 0, len(fixtures))
	for _, fixture := range fixtures {
		observations = append(observations, integrationServingFilterObservation(fixture.id, "Cross "+fixture.id, fixture.id, fixture.location, false, at))
	}
	run := publishFixtureRun(t, repository, scope, at, 1, observations, nil)
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	for _, fixture := range fixtures {
		metadataID, bodyID := servingFilterReference(t, repository, run.ID, fixture.id)
		if fixture.technologies != nil {
			resultID := insertServingTechnologyResult(t, repository, bodyID, configurationID)
			for _, technology := range fixture.technologies {
				insertServingTechnologyMention(t, repository, resultID, technology, technology)
			}
		}
		projectID := insertServingProjectResult(t, repository, metadataID, "cross-"+fixture.id)
		if _, err := repository.pool.Exec(ctx, `INSERT INTO project_tag_assignments
            (project_tag_result_id, project_key, display_name, classification)
            VALUES ($1, $2, $2, 'named_game')`, projectID, fixture.project); err != nil {
			t.Fatal(err)
		}
		familyID := insertServingFilterJobFamilyResult(t, repository, metadataID, "cross-"+fixture.id, fixture.family)
		for _, family := range []string{fixture.family, fixture.secondary} {
			if _, err := repository.pool.Exec(ctx, `INSERT INTO job_family_matches
                (job_family_result_id, family_key, match_count) VALUES ($1, $2, 1)`, familyID, family); err != nil {
				t.Fatal(err)
			}
		}
	}
	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatal(err)
	}
	condition := func(dimension jdserving.TrendDimension, op string, values ...string) jdserving.CrossCondition {
		return jdserving.CrossCondition{Dimension: dimension, Operator: op, Values: values}
	}
	shanghai := condition(jdserving.TrendDimensionLocation, "any", "上海")
	java := condition(jdserving.TrendDimensionTechnology, "any", "java")
	for _, test := range []struct {
		name  string
		query jdserving.CrossStatisticsQuery
		total int
	}{
		{"shanghai java", jdserving.CrossStatisticsQuery{Conditions: []jdserving.CrossCondition{shanghai, java}}, 2},
		{"same dimension union", jdserving.CrossStatisticsQuery{Conditions: []jdserving.CrossCondition{shanghai, condition(jdserving.TrendDimensionTechnology, "any", "java", "python")}}, 2},
		{"same dimension intersection", jdserving.CrossStatisticsQuery{Conditions: []jdserving.CrossCondition{shanghai, condition(jdserving.TrendDimensionTechnology, "all", "java", "python")}}, 1},
		{"explicit exclusion", jdserving.CrossStatisticsQuery{Conditions: []jdserving.CrossCondition{condition(jdserving.TrendDimensionTechnology, "any", "java", "python")}, Exclusions: []jdserving.CrossCondition{condition(jdserving.TrendDimensionLocation, "any", "北京")}}, 2},
		{"unrelated missing technology", jdserving.CrossStatisticsQuery{Conditions: []jdserving.CrossCondition{shanghai, condition(jdserving.TrendDimensionJobFamily, "any", "backend")}}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := serving.ReadCrossStatistics(ctx, test.query)
			if err != nil {
				t.Fatal(err)
			}
			if result.Total != test.total {
				t.Fatalf("got %d jobs, want %d", result.Total, test.total)
			}
		})
	}
	for _, primaryOnly := range []bool{false, true} {
		result, err := serving.ReadCrossStatistics(ctx, jdserving.CrossStatisticsQuery{
			Conditions: []jdserving.CrossCondition{shanghai, java}, GroupBy: jdserving.TrendDimensionJobFamily, PrimaryFamilyOnly: primaryOnly,
		})
		if err != nil {
			t.Fatal(err)
		}
		counts := make(map[string]int)
		for _, group := range result.Groups {
			counts[group.Value] = group.Count
		}
		if result.Total != 2 || counts["backend"] != 1 || counts["frontend"] != 1 {
			t.Fatalf("unexpected family counts: %#v", result)
		}
		if !primaryOnly && counts["operations"] != 2 || primaryOnly && counts["operations"] != 0 {
			t.Fatalf("secondary family mismatch (primaryOnly=%v): %#v", primaryOnly, result)
		}
	}
	for _, test := range []struct {
		dimension jdserving.TrendDimension
		counts    map[string]int
	}{
		{jdserving.TrendDimensionTechnology, map[string]int{"java": 2, "python": 1}},
		{jdserving.TrendDimensionProject, map[string]int{"project-a": 2}},
		{jdserving.TrendDimensionLocation, map[string]int{"上海": 2}},
	} {
		result, err := serving.ReadCrossStatistics(ctx, jdserving.CrossStatisticsQuery{
			Conditions: []jdserving.CrossCondition{shanghai, java}, GroupBy: test.dimension,
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.Total != 2 || len(result.Groups) != len(test.counts) {
			t.Fatalf("unexpected %s groups: %#v", test.dimension, result)
		}
		for _, group := range result.Groups {
			if test.counts[group.Value] != group.Count {
				t.Fatalf("unexpected %s group %q count %d", test.dimension, group.Value, group.Count)
			}
		}
	}
}

func TestCrossStatisticsCategorySnapshotAndUnknownExclusion(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 5, 2, 0, 0, 0, time.UTC)
	run := publishFixtureRun(t, repository, scope, at, 1, []Observation{
		integrationServingFilterObservation("engines", "Engines", "engines", "上海", false, at),
		integrationServingFilterObservation("unknown-location", "Unknown", "unknown", "", false, at),
		integrationServingFilterObservation("beijing", "Beijing", "beijing", "北京", false, at),
		integrationServingFilterObservation("missing-technology", "No technology", "missing", "上海", false, at),
	}, nil)
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	var firstResult string
	var expected []string
	for index, id := range []string{"engines", "unknown-location", "beijing"} {
		_, bodyID := servingFilterReference(t, repository, run.ID, id)
		technologyID := insertServingTechnologyResult(t, repository, bodyID, configurationID)
		if index == 0 {
			firstResult = technologyID
		}
		count := 1
		if index == 0 {
			count = 125
		}
		for n := 0; n < count; n++ {
			key := fmt.Sprintf("engine-%02d", n)
			if index == 0 {
				expected = append(expected, key)
			}
			if _, err := repository.pool.Exec(ctx, `INSERT INTO technology_identities (technology_key, canonical_name, technology_kind)
                VALUES ($1, $1, 'game-engine') ON CONFLICT (technology_key) DO NOTHING`, key); err != nil {
				t.Fatal(err)
			}
			insertServingTechnologyMention(t, repository, technologyID, key, key)
		}
	}
	for _, entry := range []struct{ id, project string }{{"engines", "project-a"}, {"beijing", "project-b"}} {
		metadataID, _ := servingFilterReference(t, repository, run.ID, entry.id)
		projectID := insertServingProjectResult(t, repository, metadataID, "cross-project-"+entry.id)
		if _, err := repository.pool.Exec(ctx, `INSERT INTO project_tag_assignments
            (project_tag_result_id, project_key, display_name, classification)
            VALUES ($1, $2, $2, 'named_game')`, projectID, entry.project); err != nil {
			t.Fatal(err)
		}
	}
	unknownMetadataID, _ := servingFilterReference(t, repository, run.ID, "unknown-location")
	insertServingProjectResult(t, repository, unknownMetadataID, "cross-project-unknown")
	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatal(err)
	}
	query := jdserving.CrossStatisticsQuery{
		Conditions:       []jdserving.CrossCondition{{Dimension: jdserving.TrendDimensionTechnology, Operator: "any", Categories: []string{"game-engines"}}},
		Exclusions:       []jdserving.CrossCondition{{Dimension: jdserving.TrendDimensionLocation, Operator: "any", Values: []string{"北京"}}},
		GroupBy:          jdserving.TrendDimensionTechnology,
		CategorySnapshot: map[string][]string{"game-engines": expected},
	}
	result, err := serving.ReadCrossStatistics(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Groups) != 125 {
		t.Fatalf("category expansion, missing technology, or unknown exclusion was mishandled: %#v", result)
	}
	unknownProject := query
	unknownProject.Exclusions = []jdserving.CrossCondition{{Dimension: jdserving.TrendDimensionProject, Operator: "any", Values: []string{"project-b"}}}
	projectFiltered, err := serving.ReadCrossStatistics(ctx, unknownProject)
	if err != nil || projectFiltered.Total != 1 {
		t.Fatalf("unknown project was treated as definitely not excluded: result=%#v err=%v", projectFiltered, err)
	}
	// A formerly confirmed category snapshot cannot silently acquire a new
	// value between discovery and aggregation.
	if _, err := repository.pool.Exec(ctx, `INSERT INTO technology_identities (technology_key, canonical_name, technology_kind)
        VALUES ('engine-125', 'engine-125', 'game-engine') ON CONFLICT (technology_key) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	insertServingTechnologyMention(t, repository, firstResult, "engine-125", "engine-125")
	if _, err := serving.ReadCrossStatistics(ctx, query); !errors.Is(err, jdserving.ErrInvalidCrossStatisticsQuery) {
		t.Fatalf("stale category snapshot returned %v, want rejection", err)
	}
	query.CategorySnapshot["game-engines"] = append(expected, "engine-125")
	result, err = serving.ReadCrossStatistics(ctx, query)
	if err != nil || result.Total != 1 || len(result.Groups) != 126 {
		t.Fatalf("new category snapshot result=%#v err=%v", result, err)
	}
	projectResult, err := serving.ReadCrossStatistics(ctx, jdserving.CrossStatisticsQuery{GroupBy: jdserving.TrendDimensionProject})
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range projectResult.Groups {
		if group.Value == "unrecognized" {
			t.Fatalf("filter-only pseudo value appeared in project ranking: %#v", projectResult)
		}
	}
	const nextDictionary = "00000000-0000-4000-8000-000000000904"
	const nextConfiguration = "00000000-0000-4000-8000-000000000905"
	if _, err := repository.pool.Exec(ctx, `INSERT INTO technology_dictionary_versions
        (id, version_label, content_sha256, status, source_document, created_by)
        VALUES ($1, 'cross-next', $2, 'candidate', 'synthetic test', 'test')`, nextDictionary, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `INSERT INTO technology_analysis_configurations
        (id, parser_version, dictionary_version_id, rule_set_sha256, status, created_by)
        VALUES ($1, 'cross-next', $2, $3, 'candidate', 'test')`, nextConfiguration, nextDictionary, strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `UPDATE technology_analysis_configurations SET status = 'superseded' WHERE id = $1`, configurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `UPDATE technology_dictionary_versions SET status = 'published', published_at = now() WHERE id = $1`, nextDictionary); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `UPDATE technology_analysis_configurations SET status = 'published', published_at = now() WHERE id = $1`, nextConfiguration); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `UPDATE technology_analysis_publication_state SET configuration_id = $1 WHERE singleton = true`, nextConfiguration); err != nil {
		t.Fatal(err)
	}
	if _, err := serving.ReadCrossStatistics(ctx, query); !errors.Is(err, jdserving.ErrInvalidCrossStatisticsQuery) {
		t.Fatalf("changed publication returned %v, want stale category rejection", err)
	}
}

func TestCrossStatisticsDoesNotCombineRecordsOrBypassExclusions(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	firstScope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	secondScope, err := repository.EnsureScope(ctx, Scope{SourceCompany: SourceCompanyMihoyo, Filter: map[string]any{"hireType": float64(1)}, FilterSHA256: strings.Repeat("1", 64)})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 4, 2, 0, 0, 0, time.UTC)
	type fixture struct{ id, location, technology, project string }
	first := []fixture{
		{"duplicate", "上海", "java", "project-a"},
		{"split", "上海", "python", "project-a"},
		{"other-project", "上海", "java", "project-a"},
		{"excluded", "上海", "java", "project-a"},
		{"pending", "上海", "java", "project-a"},
		{"closed", "上海", "java", "project-a"},
	}
	second := []fixture{
		{"duplicate", "上海", "java", "project-a"},
		{"split", "北京", "java", "project-b"},
		{"other-project", "北京", "java", "project-b"},
		{"excluded", "北京", "java", "project-b"},
	}
	makeObservations := func(fixtures []fixture, scopeName string, observedAt time.Time) []Observation {
		result := make([]Observation, 0, len(fixtures))
		for _, fixture := range fixtures {
			result = append(result, integrationServingFilterObservation(fixture.id, "Cross "+scopeName+" "+fixture.id, scopeName+"-"+fixture.id, fixture.location, false, observedAt))
		}
		return result
	}
	firstRun := publishFixtureRun(t, repository, firstScope, at, 1, makeObservations(first, "one", at), nil)
	publishFixtureRun(t, repository, firstScope, at.Add(time.Minute), 1, makeObservations(first[:4], "one", at.Add(time.Minute)), map[string]RecheckResult{
		"pending": {Outcome: RecheckIndeterminate, Code: "fixture-pending"},
		"closed":  {Outcome: RecheckDefinitelyUnavailable, Code: "fixture-closed"},
	})
	secondRun := publishFixtureRun(t, repository, secondScope, at.Add(2*time.Minute), 1, makeObservations(second, "two", at.Add(2*time.Minute)), nil)
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	for _, entry := range []struct {
		run      Run
		fixtures []fixture
	}{{firstRun, first}, {secondRun, second}} {
		for _, fixture := range entry.fixtures {
			metadataID, bodyID := servingFilterReference(t, repository, entry.run.ID, fixture.id)
			technologyID := insertServingTechnologyResult(t, repository, bodyID, configurationID)
			insertServingTechnologyMention(t, repository, technologyID, fixture.technology, fixture.technology)
			projectID := insertServingProjectResult(t, repository, metadataID, "cross-"+fixture.id)
			if _, err := repository.pool.Exec(ctx, `INSERT INTO project_tag_assignments
                (project_tag_result_id, project_key, display_name, classification)
                VALUES ($1, $2, $2, 'named_game')`, projectID, fixture.project); err != nil {
				t.Fatal(err)
			}
		}
	}
	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatal(err)
	}
	query := jdserving.CrossStatisticsQuery{
		Conditions: []jdserving.CrossCondition{
			{Dimension: jdserving.TrendDimensionLocation, Operator: "any", Values: []string{"上海"}},
			{Dimension: jdserving.TrendDimensionTechnology, Operator: "any", Values: []string{"java"}},
		},
		GroupBy: jdserving.TrendDimensionProject,
	}
	result, err := serving.ReadCrossStatistics(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 4 || len(result.Groups) != 1 || result.Groups[0].Value != "project-a" || result.Groups[0].Count != 4 {
		t.Fatalf("deduplication, split conditions, pending/closed state, or group scope is wrong: %#v", result)
	}
	query.Exclusions = []jdserving.CrossCondition{{Dimension: jdserving.TrendDimensionLocation, Operator: "any", Values: []string{"北京"}}}
	result, err = serving.ReadCrossStatistics(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Groups) != 1 || result.Groups[0].Count != 2 {
		t.Fatalf("another current record bypassed exclusion: %#v", result)
	}
}

func TestCrossStatisticsOfflineProviderEndToEnd(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)
	fixtures := []struct {
		id, location, family string
		technologies         []string
	}{
		{"a", "上海", "backend", []string{"unity", "unreal", "java", "go"}},
		{"b", "上海", "backend", []string{"unity", "java"}},
		{"c", "北京", "frontend", []string{"unreal", "java"}},
		{"d", "上海", "backend", []string{"python"}},
		{"e", "北京", "backend", []string{"unreal", "go"}},
	}
	observations := make([]Observation, 0, len(fixtures))
	for _, fixture := range fixtures {
		observations = append(observations, integrationServingFilterObservation(fixture.id, "Fixture "+fixture.id, fixture.id, fixture.location, false, at))
	}
	run := publishFixtureRun(t, repository, scope, at, 1, observations, nil)
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	for _, engine := range []string{"unity", "unreal"} {
		if _, err := repository.pool.Exec(ctx, `INSERT INTO technology_identities (technology_key, canonical_name, technology_kind)
            VALUES ($1, $2, 'game-engine') ON CONFLICT (technology_key) DO NOTHING`, engine, strings.ToUpper(engine[:1])+engine[1:]); err != nil {
			t.Fatal(err)
		}
	}
	for _, fixture := range fixtures {
		metadataID, bodyID := servingFilterReference(t, repository, run.ID, fixture.id)
		technologyID := insertServingTechnologyResult(t, repository, bodyID, configurationID)
		for _, technology := range fixture.technologies {
			insertServingTechnologyMention(t, repository, technologyID, technology, strings.ToUpper(technology[:1])+technology[1:])
		}
		familyID := insertServingFilterJobFamilyResult(t, repository, metadataID, "offline-family-"+fixture.id, fixture.family)
		for _, family := range []string{fixture.family, "operations"} {
			if _, err := repository.pool.Exec(ctx, `INSERT INTO job_family_matches
                (job_family_result_id, family_key, match_count) VALUES ($1, $2, 1)`, familyID, family); err != nil {
				t.Fatal(err)
			}
		}
	}
	serving, err := jdserving.NewRepository(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(serving.Close)
	if _, err := serving.ReconcilePublishedInventory(ctx); err != nil {
		t.Fatal(err)
	}
	category := `{"dimension":"technology","operator":"any","categories":["game-engines"]}`
	backend := `{"dimension":"job_family","operator":"any","values":["backend"]}`
	unity := `{"dimension":"technology","operator":"any","values":["unity"]}`
	unreal := `{"dimension":"technology","operator":"any","values":["unreal"]}`
	var twoAmbiguous aitrend.CrossStatisticsPlan
	locations := []aitrend.CrossConditionGroup{
		{Dimension: aitrend.DimensionLocation, Operator: aitrend.CrossAny, Values: []string{"上海"}},
		{Dimension: aitrend.DimensionLocation, Operator: aitrend.CrossAny, Values: []string{"北京"}},
		{Dimension: aitrend.DimensionLocation, Operator: aitrend.CrossAny, Values: []string{"上海", "北京"}},
		{Dimension: aitrend.DimensionLocation, Operator: aitrend.CrossAll, Values: []string{"上海", "北京"}},
	}
	technologies := []aitrend.CrossConditionGroup{
		{Dimension: aitrend.DimensionTechnology, Operator: aitrend.CrossAny, Values: []string{"java"}},
		{Dimension: aitrend.DimensionTechnology, Operator: aitrend.CrossAny, Values: []string{"python"}},
		{Dimension: aitrend.DimensionTechnology, Operator: aitrend.CrossAny, Values: []string{"java", "python"}},
		{Dimension: aitrend.DimensionTechnology, Operator: aitrend.CrossAll, Values: []string{"java", "python"}},
	}
	for locationIndex, location := range locations {
		for technologyIndex, technology := range technologies {
			twoAmbiguous.SubQueries = append(twoAmbiguous.SubQueries, aitrend.CrossStatisticsQuery{
				ID: fmt.Sprintf("q%d-%d", locationIndex, technologyIndex), Time: aitrend.CrossTimeCurrent,
				Conditions: []aitrend.CrossConditionGroup{location, technology},
			})
		}
	}
	ambiguousPayload, err := json.Marshal(twoAmbiguous)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, question, params string
		want                   []string
		curves                 []string
	}{
		{"shanghai java", "上海 Java 岗位有多少", `{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java"]}]}]}`, []string{"地点（上海）且技术关键词（Java）：2 个岗位", "不表示交集或并集"}, []string{"上海", "java"}},
		{"multiple dimension unions", "上海或北京的 Java 或 Python 岗位有多少", `{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海","北京"]},{"dimension":"technology","operator":"any","values":["java","python"]}]}]}`, []string{"地点（上海或北京）且技术关键词（Java或Python）：4 个岗位"}, []string{"上海", "北京", "java", "python"}},
		{"mixed same dimension groups", "上海的（Java 或 Python）且 Go 岗位有多少", `{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java","python"]},{"dimension":"technology","operator":"any","values":["go"]}]}]}`, []string{"地点（上海）且技术关键词（Java或Python）且技术关键词（Go）：1 个岗位"}, []string{"上海", "java", "python", "go"}},
		{"union of intersection", "（Java 且 Python）或 Go 岗位有多少", `{"sub_queries":[{"id":"q","time":"current","conditions":[{"dimension":"technology","operator":"any","values":["java","go"]},{"dimension":"technology","operator":"any","values":["python","go"]}]}]}`, []string{"技术关键词（Java或Go）且技术关键词（Python或Go）：2 个岗位"}, []string{"java", "python", "go"}},
		{"two ambiguous dimensions", "上海和北京的 Java 和 Python 岗位有多少", string(ambiguousPayload), []string{"地点（上海或北京）且技术关键词（Java或Python）：4 个岗位", "地点（上海且北京）且技术关键词（Java且Python）：0 个岗位"}, []string{"上海", "北京", "java", "python"}},
		{"engine backend", "游戏引擎的后端岗位有多少", `{"sub_queries":[{"id":"q","time":"current","conditions":[` + category + `,` + backend + `]}]}`, []string{"技术关键词（游戏引擎）且岗位族（后端）：3 个岗位"}, []string{"unity", "unreal", "backend"}},
		{"family ranking within engines", "游戏引擎相关岗位中哪些岗位族最多", `{"sub_queries":[{"id":"q","time":"current","group_by":"job_family","conditions":[` + category + `]}]}`, []string{"技术关键词（游戏引擎）：4 个岗位", "后端：3 个"}, []string{"unity", "unreal"}},
		{"ambiguous conjunction", "Unity 和 Unreal 的后端岗位有多少", `{"sub_queries":[{"id":"a","time":"current","conditions":[` + unity + `,` + backend + `]},{"id":"b","time":"current","conditions":[` + unreal + `,` + backend + `]},{"id":"u","time":"current","conditions":[{"dimension":"technology","operator":"any","values":["unity","unreal"]},` + backend + `]},{"id":"i","time":"current","conditions":[{"dimension":"technology","operator":"all","values":["unity","unreal"]},` + backend + `]}]}`, []string{"技术关键词（Unity）且岗位族（后端）：2", "技术关键词（Unreal）且岗位族（后端）：2", "技术关键词（Unity或Unreal）且岗位族（后端）：3", "技术关键词（Unity且Unreal）且岗位族（后端）：1"}, []string{"unity", "unreal", "backend"}},
		{"explicit exclusion", "游戏引擎的后端岗位排除 Unity 有多少", `{"sub_queries":[{"id":"q","time":"current","conditions":[` + category + `,` + backend + `],"exclusions":[` + unity + `]}]}`, []string{"排除技术关键词（Unity）：1 个岗位"}, []string{"unreal", "backend"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			chartValue := "unity"
			if test.name == "shanghai java" || test.name == "multiple dimension unions" || test.name == "mixed same dimension groups" || test.name == "union of intersection" || test.name == "two ambiguous dimensions" {
				chartValue = "java"
			} else if test.name == "explicit exclusion" {
				chartValue = "unreal"
			}
			summary := aitrend.ToolCall{ID: "summary", Name: aitrend.ToolBatchTrendSummary, Params: json.RawMessage(fmt.Sprintf(`[{"dimension":"technology","value":%q,"range":"month"}]`, chartValue))}
			intent := json.RawMessage(fmt.Sprintf(`{"contract_version":"ai-trend-draft-v1","sub_queries":[{"id":"chart","dimension":"technology","expressions":[%q],"ranking":{"metric":"period_end_active_jobs","order":"desc","limit":1}}],"time_range":{"kind":"month"},"output":{"series_metric":"daily_active_jobs","chart_type":"line"}}`, chartValue))
			provider := &crossFixtureProvider{turns: []aitrend.IntentToolTurn{
				{ToolCalls: []aitrend.ToolCall{{ID: "cross", Name: aitrend.ToolCrossStatistics, Params: json.RawMessage(test.params)}}},
				{ToolCalls: []aitrend.ToolCall{summary}}, {Intent: intent},
			}}
			runner, err := aitrend.NewTrendRunner(provider, serving, crossFixtureTrendReader{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			var reply strings.Builder
			curves := make(map[string]bool)
			if err := runner.Run(ctx, test.question, func(event aitrend.StreamEvent) error {
				reply.WriteString(event.Delta)
				if event.Type == aitrend.StreamEventDraft {
					for _, curve := range event.Draft.Curves {
						curves[curve.Condition.Value] = true
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			for _, expected := range test.want {
				if !strings.Contains(strings.ToLower(reply.String()), strings.ToLower(expected)) {
					t.Errorf("missing %q in %q", expected, reply.String())
				}
			}
			for _, value := range test.curves {
				if !curves[value] {
					t.Errorf("independent curve %q missing: %#v", value, curves)
				}
			}
		})
	}
}
