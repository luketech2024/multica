package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func readProjectDeployment(ctx context.Context, q *db.Queries, workspaceID, projectID pgtype.UUID) (workbench.ProjectDeploymentConfig, int64, error) {
	row, err := q.GetProjectDeployment(ctx, db.GetProjectDeploymentParams{WorkspaceID: workspaceID, ProjectID: projectID})
	if errors.Is(err, pgx.ErrNoRows) {
		return workbench.EmptyProjectDeployment(), 0, nil
	}
	if err != nil {
		return workbench.ProjectDeploymentConfig{}, 0, err
	}
	var config workbench.ProjectDeploymentConfig
	err = json.Unmarshal(row.Config, &config)
	return config, row.Revision, err
}

func (h *Handler) GetProjectDeployment(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	config, revision, err := readProjectDeployment(r.Context(), h.Queries, project.WorkspaceID, project.ID)
	if err != nil {
		writeError(w, 500, "failed to load application deployments")
		return
	}
	deployments, err := h.Queries.ListApplicationDeploymentResults(r.Context(), db.ListApplicationDeploymentResultsParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	if err != nil {
		writeError(w, 500, "failed to load deployment results")
		return
	}
	if deployments == nil {
		deployments = []db.ListApplicationDeploymentResultsRow{}
	}
	writeJSON(w, 200, map[string]any{"config": config, "revision": revision, "deployments": deployments})
}

func (h *Handler) DiscoverDeploymentHost(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.workbenchAdmin(w, r); !ok {
		return
	}
	if _, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id")); !ok {
		return
	}
	var input struct {
		Host string `json:"host"`
		Port string `json:"port"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		writeError(w, 400, "invalid server address")
		return
	}
	input.Host, input.Port = strings.TrimSpace(input.Host), strings.TrimSpace(input.Port)
	resource := workbench.DeploymentResource{Name: "Host discovery", Env: "staging", Host: input.Host, Port: input.Port}
	target := resource.Target()
	if input.Host == "" || input.Port == "" {
		writeError(w, 400, "server address and SSH port are required")
		return
	}
	if err := target.Validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	fingerprint, err := workbench.DiscoverSSHHostKey(ctx, input.Host, input.Port)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"host_key": fingerprint})
}

func (h *Handler) CheckDeploymentResource(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var resource workbench.DeploymentResource
	if chi.URLParam(r, "resourceId") == "" {
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&resource) != nil {
			writeError(w, 400, "invalid server configuration")
			return
		}
		config := workbench.EmptyProjectDeployment()
		config.Resources = []workbench.DeploymentResource{resource}
		if err := config.Validate(); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		resource = config.Resources[0]
	} else {
		resourceID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "resourceId"), "resource id")
		if !ok {
			return
		}
		config, _, err := readProjectDeployment(r.Context(), h.Queries, project.WorkspaceID, project.ID)
		if err != nil {
			writeError(w, 500, "failed to load resources")
			return
		}
		var found bool
		resource, found = applicationResource(config, uuidToString(resourceID))
		if !found {
			writeError(w, 404, "resource not found")
			return
		}
	}
	if resource.CredentialID == "" {
		writeError(w, 400, "select an SSH credential")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	key, err := h.applicationSSHSecret(ctx, db.WorkbenchReleaseOrder{WorkspaceID: project.WorkspaceID, ProjectID: project.ID}, resource)
	defer clear(key)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if err = writeWorkbenchAudit(ctx, h.Queries, project.WorkspaceID, member.UserID, "credential_used", map[string]any{"credential_id": resource.CredentialID, "usage": "deploy", "operation": "resource_connection_check"}); err != nil {
		writeError(w, 500, "failed to audit connection check")
		return
	}
	err = workbench.ExecuteDeploymentScript(ctx, resource, key, "true\n")
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	writeJSON(w, 200, map[string]any{"ok": err == nil, "detail": detail})
}

func (h *Handler) SaveProjectDeployment(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var input struct {
		Config           workbench.ProjectDeploymentConfig `json:"config"`
		ExpectedRevision int64                             `json:"expected_revision"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input) != nil || input.ExpectedRevision < 0 {
		writeError(w, 400, "invalid deployment configuration")
		return
	}
	if err := input.Config.Validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin deployment update")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	// Use the credential lock order shared by credential rotation and deletion.
	if _, err = q.LockWorkspaceForChatSessionCreate(r.Context(), member.WorkspaceID); err != nil {
		writeError(w, 409, "workspace is unavailable")
		return
	}
	if _, err = q.LockProjectForChatSessionCreate(r.Context(), db.LockProjectForChatSessionCreateParams{ID: project.ID, WorkspaceID: project.WorkspaceID}); err != nil {
		writeError(w, 409, "project is unavailable")
		return
	}
	for _, resource := range input.Config.Resources {
		if resource.CredentialID == "" {
			continue
		}
		credential, err := q.GetWorkbenchCredentialForBinding(r.Context(), db.GetWorkbenchCredentialForBindingParams{WorkspaceID: member.WorkspaceID, ID: parseUUID(resource.CredentialID)})
		if err != nil || credential.Kind != resource.CredentialKind() || !slices.Contains(credential.Usage, "deploy") || (credential.ProjectID.Valid && credential.ProjectID != project.ID) {
			writeError(w, 400, "choose an SSH deployment credential available to this project")
			return
		}
	}
	for _, app := range input.Config.Applications {
		if app.RepoID != "" {
			repo, err := q.GetProjectResourceInWorkspace(r.Context(), db.GetProjectResourceInWorkspaceParams{WorkspaceID: member.WorkspaceID, ID: parseUUID(app.RepoID)})
			if err != nil || repo.ProjectID != project.ID {
				writeError(w, 400, "repository must belong to this project")
				return
			}
		}
		for _, deployment := range app.Environments {
			if deployment.HTTPS != "existing" || deployment.CertificateID == "" {
				continue
			}
			certificate, err := q.GetDeploymentCertificate(r.Context(), db.GetDeploymentCertificateParams{WorkspaceID: member.WorkspaceID, ProjectID: project.ID, ID: parseUUID(deployment.CertificateID)})
			if err != nil {
				writeError(w, 400, "certificate must belong to this project")
				return
			}
			secret, err := workbench.OpenCredential(h.WorkbenchSecretBox, uuidToString(certificate.ID), uuidToString(member.WorkspaceID), certificate.SecretEnc, certificate.WrappedKey)
			if err != nil {
				writeError(w, 400, "certificate could not be opened")
				return
			}
			var upload workbench.CertificateUpload
			err = json.Unmarshal(secret, &upload)
			clear(secret)
			if err != nil {
				writeError(w, 400, "certificate data is unavailable")
				return
			}
			upload.Domain = deployment.Domain
			if _, _, err = workbench.ParseDeploymentCertificate(upload, time.Now()); err != nil {
				writeError(w, 400, err.Error())
				return
			}
		}
	}
	raw, _ := json.Marshal(input.Config)
	var row db.ProjectDeployment
	if input.ExpectedRevision == 0 {
		row, err = q.SaveProjectDeployment(r.Context(), db.SaveProjectDeploymentParams{WorkspaceID: member.WorkspaceID, ProjectID: project.ID, Config: raw, ExpectedRevision: 0})
	} else {
		row, err = q.UpdateProjectDeployment(r.Context(), db.UpdateProjectDeploymentParams{WorkspaceID: member.WorkspaceID, ProjectID: project.ID, Config: raw, ExpectedRevision: input.ExpectedRevision})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "deployment configuration changed; reload before saving")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to save deployments")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, member.WorkspaceID, member.UserID, "project_deployment_updated", map[string]any{"project_id": uuidToString(project.ID), "revision": row.Revision}); err != nil {
		writeError(w, 500, "failed to audit deployment update")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit deployment update")
		return
	}
	writeJSON(w, 200, map[string]any{"config": input.Config, "revision": row.Revision})
}

func (h *Handler) ListDeploymentCertificates(w http.ResponseWriter, r *http.Request) {
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	rows, err := h.Queries.ListDeploymentCertificates(r.Context(), db.ListDeploymentCertificatesParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	if err != nil {
		writeError(w, 500, "failed to list certificates")
		return
	}
	writeJSON(w, 200, map[string]any{"certificates": rows, "encryption_configured": h.WorkbenchSecretBox != nil})
}

func (h *Handler) UploadDeploymentCertificate(w http.ResponseWriter, r *http.Request) {
	member, ok := h.workbenchAdmin(w, r)
	if !ok {
		return
	}
	project, ok := h.loadProjectForResource(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if h.WorkbenchSecretBox == nil {
		writeError(w, 503, "configure credential encryption before uploading certificates")
		return
	}
	var input workbench.CertificateUpload
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 240 {
		writeError(w, 400, "certificate name, PEM certificate and private key are required")
		return
	}
	leaf, fingerprint, err := workbench.ParseDeploymentCertificate(input, time.Now())
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	id := dbid.NewV7()
	raw, _ := json.Marshal(input)
	secret, wrapped, err := workbench.SealCredential(h.WorkbenchSecretBox, uuidToString(id), uuidToString(member.WorkspaceID), string(raw))
	clear(raw)
	if err != nil {
		writeError(w, 500, "failed to encrypt certificate")
		return
	}
	domains := append([]string{}, leaf.DNSNames...)
	for _, ip := range leaf.IPAddresses {
		domains = append(domains, ip.String())
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to begin certificate upload")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	if _, err = q.LockProjectForChatSessionCreate(r.Context(), db.LockProjectForChatSessionCreateParams{ID: project.ID, WorkspaceID: member.WorkspaceID}); err != nil {
		writeError(w, 409, "project is unavailable")
		return
	}
	row, err := q.CreateDeploymentCertificate(r.Context(), db.CreateDeploymentCertificateParams{ID: id, WorkspaceID: member.WorkspaceID, ProjectID: project.ID, Name: strings.TrimSpace(input.Name), Domains: domains, NotBefore: pgtype.Timestamptz{Time: leaf.NotBefore, Valid: true}, NotAfter: pgtype.Timestamptz{Time: leaf.NotAfter, Valid: true}, Fingerprint: fingerprint, SecretEnc: secret, WrappedKey: wrapped})
	if err != nil {
		writeError(w, 500, "failed to save certificate")
		return
	}
	if err = writeWorkbenchAudit(r.Context(), q, member.WorkspaceID, member.UserID, "deployment_certificate_uploaded", map[string]any{"project_id": uuidToString(project.ID), "certificate_id": uuidToString(id), "fingerprint": fingerprint}); err != nil {
		writeError(w, 500, "failed to audit certificate upload")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "failed to commit certificate upload")
		return
	}
	writeJSON(w, 201, row)
}
