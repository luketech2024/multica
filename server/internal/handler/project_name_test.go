package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/workbench"
)

func TestProjectNamesRejectConcurrentCreateAndRename(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	results := make(chan *httptest.ResponseRecorder, 2)
	for _, title := range []string{"Concurrent unique project", "  CONCURRENT UNIQUE PROJECT  "} {
		request := newRequest("POST", "/api/projects", map[string]any{"title": title})
		go func() {
			response := httptest.NewRecorder()
			testHandler.CreateProject(response, request)
			results <- response
		}()
	}
	counts := map[int]int{}
	var created ProjectResponse
	for range 2 {
		response := <-results
		counts[response.Code]++
		if response.Code == http.StatusCreated {
			if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
		}
	}
	if created.ID != "" {
		t.Cleanup(func() {
			testutil.Call(t, testHandler.DeleteProject, testutil.WithURLParams(newRequest("DELETE", "/api/projects/"+created.ID, nil), "id", created.ID)).Want(http.StatusNoContent)
		})
	}
	if counts[http.StatusCreated] != 1 || counts[http.StatusConflict] != 1 {
		t.Fatalf("expected one create and one conflict, got %v", counts)
	}
	other := dbfx.Project(t, "Rename collision candidate")
	request := testutil.WithURLParams(newRequest("PUT", "/api/projects/"+other, map[string]any{"title": "Concurrent unique project"}), "id", other)
	testutil.Call(t, testHandler.UpdateProject, request).Want(http.StatusConflict)
}

func TestWorkbenchProjectRequiresNameAndDescription(t *testing.T) {
	if testHandler == nil {
		t.Skip("database unavailable")
	}
	for _, body := range []map[string]any{
		{"title": "A", "description": "Context", "workbench_roles": workbench.DefaultProjectRoles()},
		{"title": "Description required", "description": "  ", "workbench_roles": workbench.DefaultProjectRoles()},
	} {
		testutil.Call(t, testHandler.CreateProject, newRequest("POST", "/api/projects", body)).Want(http.StatusBadRequest)
	}
}
