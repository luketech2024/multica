package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowPlanValidatesBeforeAPI(t *testing.T) {
	cmd, _, err := issueCmd.Find([]string{"workflow", "plan"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(`{"expected_revision":1,"version":1,"idempotency_key":"plan","tasks":[{"expected":"report"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	previous, _ := cmd.Flags().GetString("file")
	t.Cleanup(func() { _ = cmd.Flags().Set("file", previous) })
	if err := cmd.Flags().Set("file", path); err != nil {
		t.Fatal(err)
	}
	err = cmd.RunE(cmd, []string{"test-issue"})
	if err == nil || !strings.Contains(err.Error(), "tasks[].expected must be an array of strings") {
		t.Fatalf("expected local field validation before API access, got %v", err)
	}
}
