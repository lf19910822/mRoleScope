package jdserving

import (
	"strings"
	"testing"
)

func TestLastModifiedSortRulesAreValidatedAndNullsLast(t *testing.T) {
	for _, value := range []string{"last_modified_asc", "last_modified_desc"} {
		rules, legacy, err := ParseListSortRules([]string{value, "title_asc"})
		if err != nil || len(rules) != 2 {
			t.Fatalf("sort %s: rules=%v err=%v", value, rules, err)
		}
		order := listJobsOrder(rules, legacy)
		if !strings.Contains(order, "last_modified") || !strings.Contains(order, "NULLS LAST") {
			t.Fatalf("sort %s order = %q", value, order)
		}
	}
	if _, _, err := ParseListSortRules([]string{"last_modified_asc", "last_modified_desc"}); err == nil {
		t.Fatal("duplicate last-modified sort must be rejected")
	}
}
