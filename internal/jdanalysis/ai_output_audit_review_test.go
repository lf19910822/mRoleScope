package jdanalysis

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestControlledAIOutputAuditTerminalBoundaryRejectsNonTTY(t *testing.T) {
	reviewed, err := RunControlledAIOutputAuditSession(
		context.Background(), nil, nil,
		[]string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}, "qwen-plus", false,
		strings.NewReader("schema_contract_mismatch\n"), &bytes.Buffer{},
	)
	if reviewed || err == nil {
		t.Fatalf("non-TTY output audit = reviewed:%t err:%v, want safe rejection", reviewed, err)
	}
}

func TestControlledAIOutputAuditTerminalScreenSanitizesUntrustedText(t *testing.T) {
	var screen bytes.Buffer
	err := writeControlledAIOutputAuditScreen(&screen, controlledAIOutputAuditTarget{
		SafeErrorCode: "invalid_response_schema",
	}, []byte("{\"input\":\"safe\\u001b[31m\"}"), []byte("{\"output\":\"unsafe\x1b[2J\u0085\"}"))
	if err != nil {
		t.Fatal(err)
	}
	got := screen.String()
	if strings.ContainsRune(got, '\x1b') || strings.ContainsRune(got, '\u0085') {
		t.Fatalf("terminal screen retained an untrusted control character: %q", got)
	}
	if !strings.Contains(got, "\\u001B") || !strings.Contains(got, "\\u0085") || !strings.Contains(got, "raw_model_output") {
		t.Fatalf("terminal screen did not safely preserve review material: %q", got)
	}
}

func TestControlledAIOutputAuditTerminalVerdictsAreFixed(t *testing.T) {
	verdict, err := readControlledOutputAuditVerdict(bufio.NewReader(strings.NewReader("schema_contract_mismatch\n")))
	if err != nil || verdict != "schema_contract_mismatch" {
		t.Fatalf("fixed audit verdict = %q, %v", verdict, err)
	}
	if _, err := readControlledOutputAuditVerdict(bufio.NewReader(strings.NewReader("free-form rationale\n"))); err == nil {
		t.Fatal("output audit accepted a free-form verdict")
	}
}

func TestControlledAIOutputAuditRecognizesHistoricalAndCurrentFrozenContracts(t *testing.T) {
	for _, target := range []controlledAIOutputAuditTarget{
		{InputContractSHA256: controlledAIInputContractSHA256(), PromptSHA256: AISystemPromptV1SHA256(), ResponseSchemaVersion: AIResponseSchemaVersionV1, ResponseSchemaSHA256: AIResponseSchemaV1SHA256()},
		{InputContractSHA256: controlledAIOutputAuditDiagnosticInputContractSHA256(), PromptSHA256: AISystemPromptV2SHA256(), ResponseSchemaVersion: AIResponseSchemaVersionV2, ResponseSchemaSHA256: AIResponseSchemaV2SHA256()},
		{InputContractSHA256: controlledAIInputContractSHA256(), PromptSHA256: AISystemPromptV3SHA256(), ResponseSchemaVersion: AIResponseSchemaVersionV3, ResponseSchemaSHA256: AIResponseSchemaV3SHA256()},
		{InputContractSHA256: controlledAIInputContractSHA256(), PromptSHA256: AISystemPromptV4SHA256(), ResponseSchemaVersion: AIResponseSchemaVersionV4, ResponseSchemaSHA256: AIResponseSchemaV4SHA256()},
	} {
		if !validControlledAIOutputAuditContract(target) {
			t.Fatalf("controlled output-audit contract was rejected: %#v", target)
		}
	}
	if validControlledAIOutputAuditContract(controlledAIOutputAuditTarget{
		InputContractSHA256:   controlledAIInputContractSHA256(),
		PromptSHA256:          AISystemPromptV2SHA256(),
		ResponseSchemaVersion: AIResponseSchemaVersionV1,
		ResponseSchemaSHA256:  AIResponseSchemaV1SHA256(),
	}) {
		t.Fatal("mixed historical/current output-audit contract was accepted")
	}
}
