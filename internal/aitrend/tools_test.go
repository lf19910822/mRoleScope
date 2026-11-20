package aitrend

import (
	"context"
	"encoding/json"
	"testing"
)

func TestToolRegistryEnforcesWhitelistAndCallLimit(t *testing.T) {
	called := 0
	registry, err := NewToolRegistry([]ToolDefinition{{
		Name: ToolRankValues,
		Handler: func(context.Context, json.RawMessage) (json.RawMessage, error) {
			called++
			return json.RawMessage(`{"ok":true}`), nil
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Call(context.Background(), ToolCall{Name: ToolListAvailableValues}); err == nil {
		t.Fatal("unregistered tool was callable")
	}
	for index := 0; index < MaxToolCalls; index++ {
		if _, err := registry.Call(context.Background(), ToolCall{Name: ToolRankValues}); err != nil {
			t.Fatalf("call %d error = %v", index+1, err)
		}
	}
	if _, err := registry.Call(context.Background(), ToolCall{Name: ToolRankValues}); err == nil {
		t.Fatal("tool call limit was not enforced")
	}
	if called != MaxToolCalls {
		t.Fatalf("handler called %d times, want %d", called, MaxToolCalls)
	}
}

func TestValidateBatchSize(t *testing.T) {
	if err := ValidateBatchSize(1); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBatchSize(MaxBatchCandidates); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBatchSize(MaxBatchCandidates + 1); err == nil {
		t.Fatal("batch size limit was not enforced")
	}
}

func TestValidateToolCallContractRejectsUnknownAndMalformedCalls(t *testing.T) {
	if err := validateToolCallContract(ToolCall{Name: "read_sql", Params: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("unknown tool was accepted")
	}
	if err := validateToolCallContract(ToolCall{Name: ToolListCategoryValues, Params: json.RawMessage(`{"category":`)}); err == nil {
		t.Fatal("malformed tool params were accepted")
	}
	if err := validateToolCallContract(ToolCall{Name: ToolListCategoryValues, Params: json.RawMessage(`{"category":"编程语言"}`)}); err != nil {
		t.Fatal(err)
	}
}
