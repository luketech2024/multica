package handler

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestProjectIssueStatsCountOnlyRootTasks(t *testing.T) {
	projectID := dbfx.Project(t, "Root task progress")
	parentID := dbfx.Issue(t, "Completed task", testutil.Cols{"project_id": projectID, "status": "done"})
	dbfx.Issue(t, "New task", testutil.Cols{"project_id": projectID, "status": "todo"})
	childID := dbfx.Issue(t, "Completed execution", testutil.Cols{"project_id": projectID, "parent_issue_id": parentID, "status": "done"})
	dbfx.Issue(t, "Nested execution", testutil.Cols{"project_id": projectID, "parent_issue_id": childID, "status": "todo"})

	var detail ProjectResponse
	testutil.Call(t, testHandler.GetProject, withURLParam(newRequest("GET", "/api/projects/"+projectID, nil), "id", projectID)).Want(http.StatusOK).JSON(&detail)
	if detail.IssueCount != 2 || detail.DoneCount != 1 {
		t.Fatalf("project detail progress = %d/%d, want 1/2 root tasks", detail.DoneCount, detail.IssueCount)
	}
	var list struct {
		Projects []ProjectResponse `json:"projects"`
	}
	testutil.Call(t, testHandler.ListProjects, newRequest("GET", "/api/projects", nil)).Want(http.StatusOK).JSON(&list)
	index := slices.IndexFunc(list.Projects, func(project ProjectResponse) bool { return project.ID == projectID })
	if index < 0 {
		t.Fatal("project missing from list")
	}
	if project := list.Projects[index]; project.IssueCount != 2 || project.DoneCount != 1 {
		t.Fatalf("project list progress = %d/%d, want 1/2 root tasks", project.DoneCount, project.IssueCount)
	}
}

func TestProjectTerminalIssueStatusKeysFallsBackToCanonicalKeys(t *testing.T) {
	h := *testHandler
	h.Queries = db.New(failQueryDBTX{
		DBTX:   testPool,
		failOn: "SELECT key FROM issue_status",
		err:    errors.New("status catalog unavailable"),
	})

	got := h.projectTerminalIssueStatusKeys(context.Background(), parseUUID(testWorkspaceID))
	want := []string{issuestatus.Done, issuestatus.Cancelled}
	if !slices.Equal(got, want) {
		t.Fatalf("project terminal keys = %v, want canonical fallback %v", got, want)
	}
}
