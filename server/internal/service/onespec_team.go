package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type OneSpecRoleSetup struct {
	Key           string  `json:"key"`
	Name          string  `json:"name"`
	Enabled       bool    `json:"enabled"`
	AgentID       *string `json:"agent_id"`
	RuntimeID     *string `json:"runtime_id"`
	Status        string  `json:"status"`
	BindingSource string  `json:"binding_source"`
}

type OneSpecTeamSetup struct {
	Status string                             `json:"status"`
	Roles  []OneSpecRoleSetup                 `json:"roles"`
	Team   workbench.ProjectRoleConfiguration `json:"team"`
}

// EnsureOneSpecWorkspaceRoles is called in the workspace creation or setup transaction.
func EnsureOneSpecWorkspaceRoles(ctx context.Context, q *db.Queries, workspaceID, userID pgtype.UUID) error {
	if err := q.LockOneSpecTeam(ctx, workspaceID); err != nil {
		return err
	}
	for _, role := range OneSpecRoles {
		if err := q.EnsureOneSpecWorkspaceRole(ctx, db.EnsureOneSpecWorkspaceRoleParams{WorkspaceID: workspaceID, RoleKey: role, ConfiguredBy: userID}); err != nil {
			return err
		}
	}
	return nil
}

func oneSpecUUID(id pgtype.UUID) *string {
	if !id.Valid {
		return nil
	}
	value := util.UUIDToString(id)
	return &value
}

// OneSpecRuntime resolves configuration only; authorization is checked at binding and invocation.
func OneSpecRuntime(ctx context.Context, q *db.Queries, workspaceID, projectID pgtype.UUID) (pgtype.UUID, error) {
	if projectID.Valid {
		config, err := q.GetOneSpecProject(ctx, db.GetOneSpecProjectParams{WorkspaceID: workspaceID, ProjectID: projectID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, err
		}
		if config.RuntimeID.Valid {
			return config.RuntimeID, nil
		}
	}
	defaults, err := q.GetOneSpecWorkspace(ctx, workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, nil
	}
	return defaults.DefaultRuntimeID, err
}

// ResolveOneSpecTeam is read-only unless prepare is true. Writers must use a transaction.
// Explicit project bindings and archived instances are never replaced by default setup.
func ResolveOneSpecTeam(ctx context.Context, q *db.Queries, workspaceID, userID pgtype.UUID, project *db.Project, prepare bool) (OneSpecTeamSetup, error) {
	result := OneSpecTeamSetup{Status: "ready", Roles: []OneSpecRoleSetup{}, Team: workbench.ProjectRoleConfiguration{Config: workbench.DefaultProjectRoles(), Revision: 1}}
	if prepare {
		if _, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: workspaceID, UserID: userID}); err != nil {
			return result, err
		}
		if err := EnsureOneSpecWorkspaceRoles(ctx, q, workspaceID, userID); err != nil {
			return result, err
		}
	}
	var resource db.ProjectResource
	var projectID pgtype.UUID
	var err error
	if project != nil {
		projectID = project.ID
		if prepare {
			if _, err = q.LockProjectWorkbenchRoles(ctx, db.LockProjectWorkbenchRolesParams{WorkspaceID: workspaceID, ID: project.ID}); err != nil {
				return result, err
			}
		}
		result.Team, resource, err = workbench.LoadProjectRoles(ctx, q, *project)
		if err != nil {
			return result, err
		}
	}
	original, err := json.Marshal(result.Team.Config)
	if err != nil {
		return result, err
	}
	runtimeID, err := OneSpecRuntime(ctx, q, workspaceID, projectID)
	if err != nil {
		return result, err
	}
	rows, err := q.ListOneSpecWorkspaceRoles(ctx, workspaceID)
	if err != nil {
		return result, err
	}
	enabled := map[string]bool{}
	for _, row := range rows {
		enabled[row.RoleKey] = row.Enabled
	}
	for _, role := range OneSpecRoles {
		entry := result.Team.Config.Roles[role]
		setup := OneSpecRoleSetup{Key: role, Name: OneSpecRoleNames[role], Enabled: entry.Enabled, AgentID: entry.AgentID, RuntimeID: entry.RuntimeID, BindingSource: "explicit"}
		automatic := entry.BindingSource == "onespec_default" || (entry.BindingSource == "" && entry.AgentID == nil && entry.RuntimeID == nil && entry.Enabled)
		if automatic {
			setup.BindingSource = "onespec_default"
			setup.RuntimeID = oneSpecUUID(runtimeID)
			setup.AgentID = nil
			active, exists := enabled[role]
			setup.Enabled = active
			switch {
			case !exists:
				setup.Status = "needs_setup"
				setup.Enabled = true
			case !active:
				setup.Status, setup.Enabled = "disabled", false
			default:
				setup.AgentID, setup.Status, err = resolveOneSpecAgent(ctx, q, workspaceID, userID, runtimeID, role, prepare)
			}
			entry = workbench.ProjectRole{Enabled: setup.Enabled, AgentID: setup.AgentID, RuntimeID: setup.RuntimeID, BindingSource: "onespec_default"}
			result.Team.Config.Roles[role] = entry
		} else {
			setup.Status, err = explicitOneSpecRoleStatus(ctx, q, workspaceID, userID, entry)
		}
		if err != nil {
			return result, err
		}
		result.Roles = append(result.Roles, setup)
		if setup.Status != "ready" && setup.Status != "disabled" && (result.Status == "ready" || result.Status == "runtime_offline") {
			result.Status = setup.Status
		}
	}
	if prepare && project != nil {
		updated, err := json.Marshal(result.Team.Config)
		if err != nil {
			return result, err
		}
		if !resource.ID.Valid || !bytes.Equal(original, updated) {
			result.Team.Revision++
			raw, err := json.Marshal(result.Team)
			if err != nil {
				return result, err
			}
			if resource.ID.Valid {
				_, err = q.UpdateProjectResource(ctx, db.UpdateProjectResourceParams{ID: resource.ID, ResourceRef: raw, Label: resource.Label, Position: resource.Position})
			} else {
				_, err = q.CreateProjectResource(ctx, db.CreateProjectResourceParams{WorkspaceID: workspaceID, ProjectID: project.ID, ResourceType: "workbench_roles", ResourceRef: raw, Label: pgtype.Text{String: "OneSpec team", Valid: true}, CreatedBy: userID})
			}
			if err != nil {
				return result, err
			}
			details, err := json.Marshal(map[string]any{"project_id": util.UUIDToString(project.ID), "runtime_id": oneSpecUUID(runtimeID), "revision": result.Team.Revision})
			if err != nil {
				return result, err
			}
			_, err = q.CreateActivity(ctx, db.CreateActivityParams{ID: dbid.NewV7(), WorkspaceID: workspaceID, ActorType: pgtype.Text{String: "member", Valid: true}, ActorID: userID, Action: "onespec_team_initialized", Details: details})
			if err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func resolveOneSpecAgent(ctx context.Context, q *db.Queries, workspaceID, userID, runtimeID pgtype.UUID, role string, prepare bool) (*string, string, error) {
	if !runtimeID.Valid {
		return nil, "pending_runtime", nil
	}
	runtime, err := q.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{ID: runtimeID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "binding_invalid", nil
	}
	if err != nil {
		return nil, "", err
	}
	if prepare {
		runtime, err = q.LockAgentRuntimeForAccessChange(ctx, runtimeID)
		if err != nil {
			return nil, "", err
		}
	}
	key := pgtype.Text{String: "onespec:" + role, Valid: true}
	agent, err := q.FindOneSpecAgent(ctx, db.FindOneSpecAgentParams{WorkspaceID: workspaceID, RuntimeID: runtimeID, SystemKey: key})
	if errors.Is(err, pgx.ErrNoRows) {
		if !RuntimeAllowsUser(runtime, userID) {
			return nil, "access_denied", nil
		}
		if !prepare {
			return nil, "needs_setup", nil
		}
		agent, err = q.CreateSystemUserAgent(ctx, db.CreateSystemUserAgentParams{WorkspaceID: workspaceID, Name: OneSpecRoleNames[role] + " · " + util.UUIDToString(runtimeID)[:8], Description: OneSpecRoleNames[role], RuntimeMode: runtime.RuntimeMode, RuntimeID: runtimeID, Visibility: "workspace", PermissionMode: "public_to", MaxConcurrentTasks: 1, OwnerID: userID, SystemKey: key})
		if err != nil {
			return nil, "", err
		}
		err = q.CreateAgentInvocationTarget(ctx, db.CreateAgentInvocationTargetParams{AgentID: agent.ID, TargetType: "workspace", TargetID: workspaceID, CreatedBy: userID})
	}
	if err != nil {
		return nil, "", err
	}
	status, err := oneSpecAgentStatus(ctx, q, workspaceID, userID, agent, runtime)
	return oneSpecUUID(agent.ID), status, err
}

func explicitOneSpecRoleStatus(ctx context.Context, q *db.Queries, workspaceID, userID pgtype.UUID, entry workbench.ProjectRole) (string, error) {
	if !entry.Enabled {
		return "disabled", nil
	}
	if entry.AgentID == nil {
		return "binding_invalid", nil
	}
	id, err := util.ParseUUID(*entry.AgentID)
	if err != nil {
		return "binding_invalid", nil
	}
	agent, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "binding_invalid", nil
	}
	if err != nil {
		return "", err
	}
	if !agent.RuntimeID.Valid || (entry.RuntimeID != nil && *entry.RuntimeID != util.UUIDToString(agent.RuntimeID)) {
		return "binding_invalid", nil
	}
	runtime, err := q.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{ID: agent.RuntimeID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "binding_invalid", nil
	}
	if err != nil {
		return "", err
	}
	return oneSpecAgentStatus(ctx, q, workspaceID, userID, agent, runtime)
}

func oneSpecAgentStatus(ctx context.Context, q *db.Queries, workspaceID, userID pgtype.UUID, agent db.Agent, runtime db.AgentRuntime) (string, error) {
	if agent.ArchivedAt.Valid {
		return "archived", nil
	}
	if !RuntimeAllowsUser(runtime, agent.OwnerID) {
		return "access_denied", nil
	}
	_, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: workspaceID, UserID: agent.OwnerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "binding_invalid", nil
	}
	if err != nil {
		return "", err
	}
	allowed := agent.OwnerID == userID
	if !allowed && agent.PermissionMode == "public_to" {
		targets, err := q.ListAgentInvocationTargets(ctx, agent.ID)
		if err != nil {
			return "", err
		}
		for _, target := range targets {
			if target.TargetType == "workspace" && target.TargetID == workspaceID || target.TargetType == "member" && target.TargetID == userID {
				allowed = true
			}
		}
	}
	if !allowed {
		return "access_denied", nil
	}
	bound, err := q.ListAgentSkills(ctx, agent.ID)
	if err != nil {
		return "", err
	}
	names := make([]AgentSkillData, 0, len(bound))
	for _, skill := range bound {
		names = append(names, AgentSkillData{Name: skill.Name})
	}
	if err = ValidateOneSpecSkillNames(names); strings.HasPrefix(agent.SystemKey.String, "onespec:") && err != nil {
		return "binding_invalid", nil
	}
	if runtime.Status != "online" {
		return "runtime_offline", nil
	}
	return "ready", nil
}
