package jdserving

import "testing"

func TestTechnologyCategoryForKindUsesControlledPublicCategories(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		wantValue string
		wantLabel string
	}{
		{
			name:      "programming language",
			kind:      "programming-language",
			wantValue: "programming-languages",
			wantLabel: "编程语言",
		},
		{
			name:      "model serving",
			kind:      "model-serving",
			wantValue: "model-serving",
			wantLabel: "模型推理服务",
		},
		{
			name:      "unknown kind does not leak",
			kind:      "future-private-kind",
			wantValue: "other",
			wantLabel: "其他技术",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			category := technologyCategoryForKind(test.kind).category
			if category.Value != test.wantValue || category.Label != test.wantLabel {
				t.Fatalf("category for %q = %#v, want value:%q label:%q", test.kind, category, test.wantValue, test.wantLabel)
			}
		})
	}
}
