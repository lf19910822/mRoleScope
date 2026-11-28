package aitrend

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCrossStatisticsContractKeepsSupportedBooleanConditions(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"same dimension union", `{"sub_queries":[{"id":"one","time":"current","conditions":[{"dimension":"technology","operator":"any","values":["java","python"]}]}]}`},
		{"same dimension intersection", `{"sub_queries":[{"id":"one","time":"current","conditions":[{"dimension":"technology","operator":"all","values":["unity","unreal-engine"]}]}]}`},
		{"cross dimension intersection", `{"sub_queries":[{"id":"one","time":"current","conditions":[{"dimension":"location","operator":"any","values":["上海"]},{"dimension":"technology","operator":"any","values":["java"]}]}]}`},
		{"mixed groups and exclusion", `{"sub_queries":[{"id":"one","time":"current","group_by":"project","ranking_limit":0,"conditions":[{"dimension":"technology","operator":"any","categories":["game-engines"]},{"dimension":"technology","operator":"all","values":["c++"]},{"dimension":"job_family","operator":"any","values":["backend"]}],"exclusions":[{"dimension":"location","operator":"any","values":["上海"]}]}]}`},
		{"historical request labels a current reference", `{"sub_queries":[{"id":"one","time":"historical_reference","conditions":[{"dimension":"project","operator":"any","values":["project-a"]}]}]}`},
		{"same-dimension ambiguity can have separate subquestions", `{"sub_queries":[{"id":"each-a","time":"current","conditions":[{"dimension":"technology","operator":"any","values":["unity"]}]},{"id":"each-b","time":"current","conditions":[{"dimension":"technology","operator":"any","values":["unreal"]}]},{"id":"union","time":"current","conditions":[{"dimension":"technology","operator":"any","values":["unity","unreal"]}]},{"id":"intersection","time":"current","conditions":[{"dimension":"technology","operator":"all","values":["unity","unreal"]}]}]}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			plan, err := DecodeCrossStatisticsPlan([]byte(test.json))
			if err != nil {
				t.Fatalf("valid plan rejected: %v", err)
			}
			var got, want any
			encoded, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(test.json), &want); err != nil {
				t.Fatal(err)
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(want)
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("contract lost conditions: got %s; want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestCrossStatisticsContractRejectsUnsupportedOrUnboundedQueries(t *testing.T) {
	cases := map[string]string{
		"cross dimension or":                         `{"sub_queries":[{"id":"one","time":"current","or":[{"dimension":"location","values":["上海"]},{"dimension":"technology","values":["java"]}]}]}`,
		"nested dimension or":                        `{"sub_queries":[{"id":"one","time":"current","conditions":[{"operator":"any","values":[{"dimension":"location","value":"上海"},{"dimension":"technology","value":"java"}]}]}]}`,
		"empty group":                                `{"sub_queries":[{"id":"one","time":"current","conditions":[{"dimension":"technology","operator":"any","values":[]}]}]}`,
		"unrestricted count":                         `{"sub_queries":[{"id":"one","time":"current"}]}`,
		"unregistered dimension":                     `{"sub_queries":[{"id":"one","time":"current","conditions":[{"dimension":"salary","operator":"any","values":["high"]}]}]}`,
		"category on different dimension":            `{"sub_queries":[{"id":"one","time":"current","conditions":[{"dimension":"location","operator":"any","categories":["game-engines"]}]}]}`,
		"category cannot mean every category member": `{"sub_queries":[{"id":"one","time":"current","conditions":[{"dimension":"technology","operator":"all","categories":["game-engines"]}]}]}`,
		"history is not a supported snapshot":        `{"sub_queries":[{"id":"one","time":"month","conditions":[{"dimension":"location","operator":"any","values":["上海"]}]}]}`,
		"invalid grouping":                           `{"sub_queries":[{"id":"one","time":"current","group_by":"salary"}]}`,
		"duplicate ids":                              `{"sub_queries":[{"id":"one","time":"current","group_by":"location"},{"id":"one","time":"current","group_by":"technology"}]}`,
		"raw sql":                                    `{"sub_queries":[{"id":"one","time":"current","group_by":"technology","sql":"SELECT * FROM jobs"}]}`,
		"extra json":                                 `{"sub_queries":[{"id":"one","time":"current","group_by":"location"}]} {}`,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeCrossStatisticsPlan([]byte(data)); err == nil {
				t.Fatalf("unsupported contract was accepted: %s", data)
			}
		})
	}
}

func TestCrossStatisticsContractBoundsValuesAndPages(t *testing.T) {
	group := CrossConditionGroup{Dimension: DimensionTechnology, Operator: CrossAny, Values: []string{strings.Repeat("x", MaxCrossValueRunes+1)}}
	query := CrossStatisticsQuery{ID: "q", Time: CrossTimeCurrent, Conditions: []CrossConditionGroup{group}}
	if err := ValidateCrossStatisticsQuery(query); err == nil {
		t.Fatal("unbounded value was accepted")
	}
	query.Conditions[0].Values = []string{"java"}
	query.PageSize = MaxCrossPageSize + 1
	if err := ValidateCrossStatisticsQuery(query); err == nil {
		t.Fatal("unbounded page size was accepted")
	}
}
