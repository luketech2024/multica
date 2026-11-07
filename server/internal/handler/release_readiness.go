package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func readReleasePullRequests(ctx context.Context, q *db.Queries, issue db.Issue) ([]workbench.ReleasePullRequest, error) {
	rows, err := q.ListWorkbenchReleasePullRequests(ctx, db.ListWorkbenchReleasePullRequestsParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
	if err != nil {
		return nil, err
	}
	prs := make([]workbench.ReleasePullRequest, 0, len(rows))
	for _, row := range rows {
		prs = append(prs, workbench.ReleasePullRequest{IssueID: uuidToString(row.IssueID), ID: uuidToString(row.PrID), RepoURL: row.RepoUrl, URL: row.HtmlUrl, State: row.State, BaseBranch: row.BaseBranch, HeadSHA: row.HeadSha, MergeCommitSHA: row.MergeCommitSha})
	}
	return prs, nil
}

func (h *Handler) GetIssueReleaseReadiness(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if issue.ParentIssueID.Valid || !issue.ProjectID.Valid {
		writeError(w, http.StatusConflict, "release readiness requires a project parent task")
		return
	}
	tx, err := h.beginWakeupWrite(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin release review")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	current, err := q.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	result, err := readIssueReleaseReadiness(r.Context(), q, current)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read release evidence")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func readIssueReleaseReadiness(ctx context.Context, q *db.Queries, current db.Issue) (workbench.ReleaseReadiness, error) {
	criteria, err := workbench.ParseRequirements(current.AcceptanceCriteria)
	if err != nil {
		return workbench.ReleaseReadiness{}, err
	}
	review, err := readDeliveryReview(ctx, q, current, criteria)
	if err != nil {
		return workbench.ReleaseReadiness{}, err
	}
	resources, err := q.ListProjectResourcesInWorkspace(ctx, db.ListProjectResourcesInWorkspaceParams{ProjectID: current.ProjectID, WorkspaceID: current.WorkspaceID})
	if err != nil {
		return workbench.ReleaseReadiness{}, err
	}
	repos := make([]workbench.ReleaseRepository, 0, len(resources))
	for _, resource := range resources {
		if resource.ResourceType != "github_repo" {
			continue
		}
		var ref githubRepoRef
		if json.Unmarshal(resource.ResourceRef, &ref) != nil {
			continue
		}
		branch := ref.Ref
		if branch == "" {
			branch = ref.DefaultBranchHint
		}
		repos = append(repos, workbench.ReleaseRepository{ID: uuidToString(resource.ID), URL: ref.URL, Branch: branch})
	}
	prs, err := readReleasePullRequests(ctx, q, current)
	if err != nil {
		return workbench.ReleaseReadiness{}, err
	}
	result := workbench.CheckReleaseReadiness(uuidToString(current.ID), current.Status, criteria, repos, prs, len(review.Blockers) == 0)
	result.Revision = current.Revision
	return result, nil
}
