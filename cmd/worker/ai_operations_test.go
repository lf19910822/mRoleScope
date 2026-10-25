package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestParseAIAnalysisOperationAllowsOnlyFixedActions(t *testing.T) {
	for _, arguments := range [][]string{
		{"prepare-trial"},
		{"run-plus-trial"},
		{"run-flash-trial"},
		{"expand-plus"},
		{"publish-plus"},
		{"status"},
	} {
		if _, err := parseAIAnalysisOperation(arguments); err != nil {
			t.Fatalf("fixed AI operation %q rejected: %v", arguments, err)
		}
	}

	for _, arguments := range [][]string{
		nil,
		{"qwen-plus"},
		{"run-plus-trial", "qwen-flash"},
		{"run-plus-trial", "--prompt", "anything"},
		{"run-plus-trial", "--sql", "select 1"},
		{"run-plus-trial", "--api-key", "not-a-key"},
		{"run-plus-trial", "--body-id", "00000000-0000-4000-8000-000000000001"},
	} {
		if _, err := parseAIAnalysisOperation(arguments); err == nil {
			t.Fatalf("uncontrolled AI arguments were accepted: %q", arguments)
		}
	}
}

func TestParseAIReviewModelAllowsOnlyFixedTrialArms(t *testing.T) {
	for _, arguments := range [][]string{{"plus"}, {"flash"}} {
		if _, err := parseAIReviewModel(arguments); err != nil {
			t.Fatalf("fixed AI review arm %q rejected: %v", arguments, err)
		}
	}
	for _, arguments := range [][]string{
		nil,
		{"qwen-plus"},
		{"plus", "--body-id", "00000000-0000-4000-8000-000000000001"},
		{"plus", "--sql", "select 1"},
		{"plus", "--reviewer", "someone"},
	} {
		if _, err := parseAIReviewModel(arguments); err == nil {
			t.Fatalf("uncontrolled AI review arguments were accepted: %q", arguments)
		}
	}
}

func TestRunAIAnalysisTrialWithoutCredentialFailsBeforeDatabaseInitialization(t *testing.T) {
	if os.Getenv("GO_WANT_AI_MISSING_CREDENTIAL_HELPER") == "1" {
		runAIAnalysis([]string{"run-plus-trial"})
		return
	}

	t.Setenv("MROLESCOPE_QWEN_API_KEY", "")
	t.Setenv("MROLESCOPE_QWEN_WORKSPACE_ID", "llm-mrolescope")
	t.Setenv("DATABASE_URL", "")
	command := exec.Command(os.Args[0], "-test.run=TestRunAIAnalysisTrialWithoutCredentialFailsBeforeDatabaseInitialization")
	command.Env = append(os.Environ(), "GO_WANT_AI_MISSING_CREDENTIAL_HELPER=1")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("missing model credential unexpectedly allowed a controlled trial")
	}
	if !strings.Contains(string(output), "AI model credential is not configured") {
		t.Fatalf("missing-credential failure did not use its safe message: %s", output)
	}
	if strings.Contains(string(output), "invalid AI analysis configuration") {
		t.Fatalf("worker initialized database configuration before rejecting missing credential: %s", output)
	}
}

func TestRunAIAnalysisRequiresWorkspaceBeforeDatabaseInitialization(t *testing.T) {
	if os.Getenv("GO_WANT_AI_MISSING_WORKSPACE_HELPER") == "1" {
		runAIAnalysis([]string{"prepare-trial"})
		return
	}

	t.Setenv("MROLESCOPE_QWEN_WORKSPACE_ID", "")
	t.Setenv("DATABASE_URL", "")
	command := exec.Command(os.Args[0], "-test.run=TestRunAIAnalysisRequiresWorkspaceBeforeDatabaseInitialization")
	command.Env = append(os.Environ(), "GO_WANT_AI_MISSING_WORKSPACE_HELPER=1")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("missing model Workspace unexpectedly allowed a controlled operation")
	}
	if !strings.Contains(string(output), "AI model workspace is not configured") {
		t.Fatalf("missing-Workspace failure did not use its safe message: %s", output)
	}
	if strings.Contains(string(output), "invalid AI analysis configuration") {
		t.Fatalf("worker initialized database configuration before rejecting missing Workspace: %s", output)
	}
}
