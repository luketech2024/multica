package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) ExecuteApplicationRelease(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "releaseId"), "release id")
	if !ok {
		return
	}
	var input struct {
		ExpectedRevision int64 `json:"expected_revision"`
		Acknowledged     bool  `json:"acknowledged"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || input.ExpectedRevision < 1 {
		writeError(w, 400, "invalid execution request")
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin execution")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	order, err := q.GetWorkbenchRelease(r.Context(), db.GetWorkbenchReleaseParams{WorkspaceID: member.WorkspaceID, ID: id})
	if err != nil {
		writeError(w, 404, "release not found")
		return
	}
	if _, err = q.LockProjectForChatSessionCreate(r.Context(), db.LockProjectForChatSessionCreateParams{ID: order.ProjectID, WorkspaceID: member.WorkspaceID}); err != nil {
		writeError(w, 409, "project is unavailable")
		return
	}
	running, err := q.ProjectReleaseExecuting(r.Context(), db.ProjectReleaseExecutingParams{WorkspaceID: member.WorkspaceID, ProjectID: order.ProjectID, Env: order.Env})
	if err != nil {
		writeError(w, 500, "failed to check active releases")
		return
	}
	if running {
		writeError(w, 409, "a release is already executing in this project environment")
		return
	}
	var manifest workbench.ReleaseManifest
	if json.Unmarshal(order.Manifest, &manifest) != nil || manifest.Delivery == nil {
		writeError(w, 400, "this release does not contain application changes")
		return
	}
	delivery := manifest.Delivery
	if order.Status != "draft" && !input.Acknowledged {
		writeError(w, 400, "verify actual server and database state before resuming")
		return
	}
	gates, err := readReleaseGates(r.Context(), q, order)
	if err != nil {
		writeError(w, 500, "failed to evaluate release checks")
		return
	}
	for _, gate := range gates {
		if gate.Required && !gate.Passed {
			writeJSON(w, 409, map[string]any{"error": "resolve release checks before execution", "gates": gates})
			return
		}
	}
	if order.Status == "draft" {
		config, _, err := readProjectDeployment(r.Context(), q, order.WorkspaceID, order.ProjectID)
		if err != nil {
			writeError(w, 500, "failed to snapshot deployments")
			return
		}
		delivery.Snapshot = &config
		delivery.Results = []workbench.ReleaseStepResult{}
		for _, change := range delivery.Changes {
			for _, app := range config.Applications {
				if app.ID != change.ApplicationID {
					continue
				}
				nodes := app.Environments[order.Env].Nodes
				if change.Type == "sql" {
					nodes = nodes[:1]
				}
				for _, node := range nodes {
					delivery.Results = append(delivery.Results, workbench.ReleaseStepResult{ID: change.ID, NodeID: node, Status: "pending"})
				}
			}
		}
	}
	if delivery.Snapshot == nil {
		writeError(w, 409, "release snapshot is missing")
		return
	}
	for _, change := range delivery.Changes {
		for _, app := range delivery.Snapshot.Applications {
			if app.ID != change.ApplicationID {
				continue
			}
			config := app.Environments[order.Env]
			certificate, err := h.applicationCertificate(r.Context(), order, config)
			if err != nil {
				writeError(w, 400, err.Error())
				return
			}
			if _, err = workbench.ApplicationDeploymentScript(uuidToString(order.ID), app, config, change, certificate); err != nil {
				writeError(w, 400, err.Error())
				return
			}
			for _, node := range config.Nodes {
				resource, exists := applicationResource(*delivery.Snapshot, node)
				if !exists || resource.Host == "" || resource.Port == "" || resource.Username == "" || resource.HostKey == "" || resource.CredentialID == "" {
					writeError(w, 400, "complete SSH connection configuration before execution")
					return
				}
				key, err := h.applicationSSHSecret(r.Context(), order, resource)
				clear(key)
				if err != nil {
					writeError(w, 400, err.Error())
					return
				}
			}
		}
	}
	for i := range delivery.Results {
		if delivery.Results[i].Status != "success" {
			delivery.Results[i].Status = "pending"
			delivery.Results[i].Detail = ""
		}
	}
	raw, _ := json.Marshal(manifest)
	order, err = q.StartApplicationRelease(r.Context(), db.StartApplicationReleaseParams{WorkspaceID: member.WorkspaceID, ID: id, Manifest: raw, ExpectedRevision: input.ExpectedRevision})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "release changed or cannot execute in its current state")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to reserve execution")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, member.WorkspaceID, member.UserID, "application_release_started", map[string]any{"release_id": uuidToString(id), "resume_acknowledged": input.Acknowledged}); err != nil {
		writeError(w, 500, "failed to audit execution")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit execution")
		return
	}
	go h.runApplicationRelease(order, manifest)
	writeJSON(w, http.StatusAccepted, releaseOrderResponse(order))
}

func applicationResource(config workbench.ProjectDeploymentConfig, id string) (workbench.DeploymentResource, bool) {
	for _, resource := range config.Resources {
		if resource.ID == id {
			return resource, true
		}
	}
	return workbench.DeploymentResource{}, false
}

func (h *Handler) applicationSSHSecret(ctx context.Context, order db.WorkbenchReleaseOrder, resource workbench.DeploymentResource) ([]byte, error) {
	credential, err := h.Queries.GetWorkbenchCredentialForExecution(ctx, db.GetWorkbenchCredentialForExecutionParams{WorkspaceID: order.WorkspaceID, ID: parseUUID(resource.CredentialID)})
	if err != nil || credential.Kind != resource.CredentialKind() || !slices.Contains(credential.Usage, "deploy") || (credential.ProjectID.Valid && credential.ProjectID != order.ProjectID) || (credential.ExpiresAt.Valid && !credential.ExpiresAt.Time.After(time.Now())) {
		return nil, errors.New("SSH credential is unavailable, expired or outside this project")
	}
	return workbench.OpenCredential(h.WorkbenchSecretBox, uuidToString(credential.ID), uuidToString(order.WorkspaceID), credential.SecretEnc, credential.WrappedKey)
}

func (h *Handler) applicationCertificate(ctx context.Context, order db.WorkbenchReleaseOrder, config workbench.ApplicationDeployment) (*workbench.CertificateUpload, error) {
	if config.HTTPS != "existing" {
		return nil, nil
	}
	if config.CertificateID == "" {
		return nil, errors.New("choose an existing certificate")
	}
	row, err := h.Queries.GetDeploymentCertificate(ctx, db.GetDeploymentCertificateParams{WorkspaceID: order.WorkspaceID, ProjectID: order.ProjectID, ID: parseUUID(config.CertificateID)})
	if err != nil {
		return nil, errors.New("certificate is unavailable")
	}
	secret, err := workbench.OpenCredential(h.WorkbenchSecretBox, uuidToString(row.ID), uuidToString(order.WorkspaceID), row.SecretEnc, row.WrappedKey)
	if err != nil {
		return nil, err
	}
	defer clear(secret)
	var certificate workbench.CertificateUpload
	if json.Unmarshal(secret, &certificate) != nil {
		return nil, errors.New("certificate data is invalid")
	}
	certificate.Domain = config.Domain
	if _, _, err = workbench.ParseDeploymentCertificate(certificate, time.Now()); err != nil {
		return nil, err
	}
	return &certificate, nil
}

func (h *Handler) runApplicationRelease(order db.WorkbenchReleaseOrder, manifest workbench.ReleaseManifest) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	delivery := manifest.Delivery
	persist := func(status string) bool {
		raw, _ := json.Marshal(manifest)
		writeContext, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		saved, err := h.Queries.UpdateReleaseExecution(writeContext, db.UpdateReleaseExecutionParams{WorkspaceID: order.WorkspaceID, ID: order.ID, Manifest: raw, Status: status, ExpectedRevision: order.Revision})
		if err != nil {
			return false
		}
		order = saved
		return true
	}
	for changeIndex := range delivery.Changes {
		change := delivery.Changes[changeIndex]
		blocked := false
		for _, result := range delivery.Results {
			if result.ID == change.After && result.Status != "success" {
				blocked = true
			}
		}
		var app workbench.DeploymentApplication
		for _, candidate := range delivery.Snapshot.Applications {
			if candidate.ID == change.ApplicationID {
				app = candidate
				break
			}
		}
		config := app.Environments[order.Env]
		indices := []int{}
		for i, result := range delivery.Results {
			if result.ID != change.ID || result.Status == "success" {
				continue
			}
			if blocked {
				delivery.Results[i].Status = "blocked"
				delivery.Results[i].Detail = "dependency did not complete"
			} else {
				indices = append(indices, i)
			}
		}
		if blocked {
			if !persist("deploying") {
				return
			}
			continue
		}
		var localArtifact workbench.LocalReleaseArtifact
		if len(indices) > 0 && change.Type == "app" && app.SourceKind == "git" {
			operation, stop := context.WithTimeout(ctx, 10*time.Minute)
			var err error
			localArtifact, err = workbench.PrepareLocalReleaseArtifact(operation, uuidToString(order.ID), app, change)
			stop()
			if err != nil {
				for _, i := range indices {
					delivery.Results[i].Status = "failed"
					delivery.Results[i].Detail = err.Error()
					delivery.Results[i].At = time.Now().UTC().Format(time.RFC3339)
				}
				if !persist("deploying") {
					return
				}
				continue
			}
			change.ResolvedVersion, change.ArtifactDigest = localArtifact.Commit, localArtifact.Digest
			delivery.Changes[changeIndex] = change
			if !persist("deploying") {
				return
			}
		}
		batch := 1
		if len(indices) > 0 && change.Type == "app" && change.ResolvedVersion == "" {
			resource, _ := applicationResource(*delivery.Snapshot, delivery.Results[indices[0]].NodeID)
			operation, stop := context.WithTimeout(ctx, 5*time.Minute)
			key, resolveErr := h.applicationSSHSecret(operation, order, resource)
			if resolveErr == nil {
				resolveErr = writeWorkbenchAudit(operation, h.Queries, order.WorkspaceID, order.CreatedBy, "credential_used", map[string]any{"release_id": uuidToString(order.ID), "credential_id": resource.CredentialID, "usage": "deploy", "operation": "resolve_version"})
			}
			if resolveErr == nil {
				change.ResolvedVersion, resolveErr = workbench.ResolveApplicationVersion(operation, resource, key, app, change.Version)
			}
			clear(key)
			stop()
			if resolveErr != nil {
				for _, i := range indices {
					delivery.Results[i].Status = "failed"
					delivery.Results[i].Detail = resolveErr.Error()
				}
				if !persist("deploying") {
					return
				}
				continue
			}
			delivery.Changes[changeIndex] = change
			if !persist("deploying") {
				return
			}
		}
		if config.Strategy == "batch" && change.Type == "app" {
			batch = 2
		}
		for start := 0; start < len(indices); start += batch {
			end := min(start+batch, len(indices))
			group := indices[start:end]
			for _, i := range group {
				delivery.Results[i].Status = "running"
				delivery.Results[i].At = time.Now().UTC().Format(time.RFC3339)
			}
			if !persist("deploying") {
				return
			}
			var workers sync.WaitGroup
			for _, i := range group {
				workers.Add(1)
				go func(index int) {
					defer workers.Done()
					result := &delivery.Results[index]
					resource, _ := applicationResource(*delivery.Snapshot, result.NodeID)
					operation, stop := context.WithTimeout(ctx, 5*time.Minute)
					defer stop()
					key, err := h.applicationSSHSecret(operation, order, resource)
					defer clear(key)
					var certificate *workbench.CertificateUpload
					if err == nil {
						certificate, err = h.applicationCertificate(operation, order, config)
					}
					var script string
					if err == nil {
						script, err = workbench.ApplicationDeploymentScript(uuidToString(order.ID), app, config, change, certificate)
					}
					if err == nil {
						err = writeWorkbenchAudit(operation, h.Queries, order.WorkspaceID, order.CreatedBy, "credential_used", map[string]any{"release_id": uuidToString(order.ID), "credential_id": resource.CredentialID, "usage": "deploy", "operation": "release_execution"})
					}
					if err == nil {
						err = workbench.CheckApplicationDeploymentTools(operation, resource, key, app, config, change)
					}
					if err == nil && localArtifact.File != "" {
						err = workbench.UploadLocalReleaseArtifact(operation, resource, key, config.Directory, uuidToString(order.ID), localArtifact)
						if err != nil {
							err = fmt.Errorf("artifact upload failed: %w", err)
						}
					}
					if err == nil {
						err = workbench.ExecuteDeploymentScript(operation, resource, key, script)
					}
					result.At = time.Now().UTC().Format(time.RFC3339)
					if err != nil {
						result.Status = "failed"
						if change.Type != "app" {
							result.Status = "unknown"
						}
						result.Detail = err.Error()
					} else {
						result.Status = "success"
						result.Detail = "remote command completed"
					}
				}(i)
			}
			workers.Wait()
			failed := false
			for _, i := range group {
				if delivery.Results[i].Status != "success" {
					failed = true
				}
			}
			if !persist("deploying") {
				return
			}
			if failed {
				for _, i := range indices[end:] {
					delivery.Results[i].Status = "blocked"
					delivery.Results[i].Detail = "an earlier node did not complete"
				}
				break
			}
		}
	}
	success, failed := 0, 0
	for _, result := range delivery.Results {
		if result.Status == "success" {
			success++
		} else {
			failed++
		}
	}
	status := "live"
	if failed > 0 {
		status = "failed"
		if success > 0 {
			status = "partial"
		}
	}
	if persist(status) {
		auditContext, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = writeWorkbenchAudit(auditContext, h.Queries, order.WorkspaceID, order.CreatedBy, "application_release_finished", map[string]any{"release_id": uuidToString(order.ID), "status": status})
	}
}
