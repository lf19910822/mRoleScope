package jdanalysis

import "testing"

func TestParseProjectTitleMatchesReviewedDictionaryEntries(t *testing.T) {
	tests := []struct {
		name           string
		title          string
		wantProjectKey string
		wantName       string
		wantClass      ProjectTagClassification
		wantKeyword    string
		wantRange      CodePointRange
	}{
		{name: "Varsapura case insensitive", title: "【vArSaPuRa】客户端", wantProjectKey: "game.varsapura", wantName: "Varsapura", wantClass: ProjectTagNamedGame, wantKeyword: "Varsapura", wantRange: CodePointRange{Start: 1, End: 10}},
		{name: "yuan chu zhi jie", title: "源初之结-客户端", wantProjectKey: "game.yuanchu-zhijie", wantName: "源初之结", wantClass: ProjectTagNamedGame, wantKeyword: "源初之结", wantRange: CodePointRange{Start: 0, End: 4}},
		{name: "xing bu gu di", title: "星布谷地-服务端", wantProjectKey: "game.xingbu-gudi", wantName: "星布谷地", wantClass: ProjectTagNamedGame, wantKeyword: "星布谷地", wantRange: CodePointRange{Start: 0, End: 4}},
		{name: "zenless", title: "绝区零-引擎", wantProjectKey: "game.zenless-zone-zero", wantName: "绝区零", wantClass: ProjectTagNamedGame, wantKeyword: "绝区零", wantRange: CodePointRange{Start: 0, End: 3}},
		{name: "honkai nexus anima", title: "崩坏：因缘精灵-战斗", wantProjectKey: "game.honkai-nexus-anima", wantName: "崩坏：因缘精灵", wantClass: ProjectTagNamedGame, wantKeyword: "崩坏：因缘精灵", wantRange: CodePointRange{Start: 0, End: 7}},
		{name: "genshin", title: "原神-图形", wantProjectKey: "game.genshin-impact", wantName: "原神", wantClass: ProjectTagNamedGame, wantKeyword: "原神", wantRange: CodePointRange{Start: 0, End: 2}},
		{name: "honkai star rail", title: "崩坏：星穹铁道-角色", wantProjectKey: "game.honkai-star-rail", wantName: "崩坏：星穹铁道", wantClass: ProjectTagNamedGame, wantKeyword: "崩坏：星穹铁道", wantRange: CodePointRange{Start: 0, End: 7}},
		{name: "honkai impact", title: "崩坏3-引擎", wantProjectKey: "game.honkai-impact-3rd", wantName: "崩坏3", wantClass: ProjectTagNamedGame, wantKeyword: "崩坏3", wantRange: CodePointRange{Start: 0, End: 3}},
		{name: "houkai gakuen", title: "崩坏学园2-客户端", wantProjectKey: "game.houkai-gakuen-2nd", wantName: "崩坏学园2", wantClass: ProjectTagNamedGame, wantKeyword: "崩坏学园2", wantRange: CodePointRange{Start: 0, End: 5}},
		{name: "tears of themis", title: "未定事件簿-服务端", wantProjectKey: "game.tears-of-themis", wantName: "未定事件簿", wantClass: ProjectTagNamedGame, wantKeyword: "未定事件簿", wantRange: CodePointRange{Start: 0, End: 5}},
		{name: "ue5 research direction", title: "UE5写实射击预研-战斗", wantProjectKey: "direction.ue5-realistic-shooter-research", wantName: "UE5写实射击预研", wantClass: ProjectTagSourceDirection, wantKeyword: "UE5写实射击预研", wantRange: CodePointRange{Start: 0, End: 9}},
		{name: "foundation model direction", title: "基模算法工程师", wantProjectKey: "direction.foundation-model", wantName: "基模", wantClass: ProjectTagSourceDirection, wantKeyword: "基模", wantRange: CodePointRange{Start: 0, End: 2}},
		{name: "research project direction", title: "预研项目-技术策划", wantProjectKey: "direction.research-project", wantName: "预研项目", wantClass: ProjectTagSourceDirection, wantKeyword: "预研项目", wantRange: CodePointRange{Start: 0, End: 4}},
		{name: "ai engine direction", title: "AI引擎-平台", wantProjectKey: "direction.ai-engine", wantName: "AI引擎", wantClass: ProjectTagSourceDirection, wantKeyword: "AI引擎", wantRange: CodePointRange{Start: 0, End: 4}},
		{name: "sandbox research direction", title: "沙盒预研-程序", wantProjectKey: "direction.sandbox-research", wantName: "沙盒预研", wantClass: ProjectTagSourceDirection, wantKeyword: "沙盒预研", wantRange: CodePointRange{Start: 0, End: 4}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ParseProjectTitle(test.title)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Assignments) != 1 {
				t.Fatalf("assignments = %#v, want exactly one", result.Assignments)
			}
			assignment := result.Assignments[0]
			if assignment.ProjectKey != test.wantProjectKey || assignment.DisplayName != test.wantName || assignment.Classification != test.wantClass {
				t.Fatalf("assignment = %#v, want key:%q name:%q classification:%q", assignment, test.wantProjectKey, test.wantName, test.wantClass)
			}
			if len(assignment.Evidence) != 1 {
				t.Fatalf("assignment evidence = %#v, want one match", assignment.Evidence)
			}
			evidence := assignment.Evidence[0]
			if evidence.MatchedKeyword != test.wantKeyword || evidence.CodePointRange != test.wantRange {
				t.Fatalf("evidence = %#v, want keyword:%q range:%#v", evidence, test.wantKeyword, test.wantRange)
			}
		})
	}
}

func TestParseProjectTitlePreservesMultipleMatchesAndNoMatch(t *testing.T) {
	multiple, err := ParseProjectTitle("绝区零-基模算法")
	if err != nil {
		t.Fatal(err)
	}
	if len(multiple.Assignments) != 2 {
		t.Fatalf("multiple assignments = %#v, want two", multiple.Assignments)
	}
	if multiple.Assignments[0].ProjectKey != "game.zenless-zone-zero" || multiple.Assignments[0].Evidence[0].CodePointRange != (CodePointRange{Start: 0, End: 3}) {
		t.Fatalf("first assignment = %#v", multiple.Assignments[0])
	}
	if multiple.Assignments[1].ProjectKey != "direction.foundation-model" || multiple.Assignments[1].Evidence[0].CodePointRange != (CodePointRange{Start: 4, End: 6}) {
		t.Fatalf("second assignment = %#v", multiple.Assignments[1])
	}

	empty, err := ParseProjectTitle("通用客户端工程师")
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Assignments) != 0 {
		t.Fatalf("no-match assignments = %#v, want empty", empty.Assignments)
	}
}

func TestParseProjectTitleDoesNotMatchEnglishKeywordInsideIdentifier(t *testing.T) {
	result, err := ParseProjectTitle("VarsapuraTools-客户端")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assignments) != 0 {
		t.Fatalf("identifier substring assignments = %#v, want empty", result.Assignments)
	}
}

func TestParseProjectTitlePreservesReviewedBoundaryAndCatalogScope(t *testing.T) {
	chineseAdjacent, err := ParseProjectTitle("Varsapura客户端")
	if err != nil {
		t.Fatal(err)
	}
	if len(chineseAdjacent.Assignments) != 1 || chineseAdjacent.Assignments[0].ProjectKey != "game.varsapura" {
		t.Fatalf("Chinese-adjacent title = %#v, want Varsapura assignment", chineseAdjacent)
	}

	for _, title := range []string{"星铁-客户端", "崩坏-图形", "社会招聘"} {
		result, err := ParseProjectTitle(title)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Assignments) != 0 {
			t.Fatalf("unregistered title %q assignments = %#v, want empty", title, result.Assignments)
		}
	}

	multiple, err := ParseProjectTitle("崩坏3和崩坏学园2-客户端")
	if err != nil {
		t.Fatal(err)
	}
	if len(multiple.Assignments) != 2 || multiple.Assignments[0].ProjectKey != "game.honkai-impact-3rd" || multiple.Assignments[1].ProjectKey != "game.houkai-gakuen-2nd" {
		t.Fatalf("distinct catalog assignments = %#v, want both reviewed games", multiple.Assignments)
	}
}
