import {
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { api } from "../api";
import { issueKeys } from "../issues/queries";
import { inboxKeys } from "../inbox/queries";
import { projectResourceKeys } from "../projects/resource-queries";
import type {
  DeployTargetInput,
  CredentialInput,
  ProjectRoles,
  ProjectTemplate,
  BusinessTemplate,
  DeployTarget,
} from "./schemas";
export * from "./schemas";
export { workbenchPipeline } from "./pipeline";
export { workbenchNextStep } from "./next-step";
export type { WorkbenchDestination } from "./next-step";
export type { PipelineState } from "./pipeline";
export type { DeliveryEvidence, DeliveryEvidenceInput, DeliveryReviewInput } from "./delivery";
export { prefillTaskTemplate } from "./template-prefill";
export type { ExecutionPlanInput, ExecutionPlanTask } from "./execution-plan";
export { parseWorkbenchHandoff, analyzeExecutionPlan } from "./execution-plan";
export { previewPropertyName, previewPath, previewURL, validPreviewURL } from "./preview";
export type { WorkbenchHandoff } from "./execution-plan";
export type { ExecutionControlAction, ExecutionPause } from "./execution-control";

export function repositoryStackCatalogOptions(wsId: string) {
  return queryOptions({queryKey: ["repository-stacks", wsId], queryFn: () => api.listRepositoryStacks()});
}
export function repositoryProfileOptions(wsId: string, projectId: string, repositoryId: string) {
  return queryOptions({queryKey: [...projectResourceKeys.list(wsId, projectId), repositoryId, "workbench-profile"], queryFn: () => api.getRepositoryProfile(projectId, repositoryId)});
}
export function useSaveRepositoryProfile(wsId: string, projectId: string, repositoryId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: import("./schemas").RepositoryProfileInput) => api.saveRepositoryProfile(projectId, repositoryId, input),
    onSuccess: (profile) => {
      qc.setQueryData(repositoryProfileOptions(wsId, projectId, repositoryId).queryKey, profile);
      qc.invalidateQueries({queryKey: projectResourceKeys.list(wsId, projectId)});
    },
  });
}

export function useControlIssueExecution(wsId: string, issueId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ action, revision }: { action: import("./execution-control").ExecutionControlAction; revision?: number }) => {
      if (revision === undefined) {
        const current = await api.getIssue(issueId);
        if (action === "pause" && current.workbench_pause) return current;
        revision = current.revision;
      }
      if (!revision) throw new Error("Task revision is unavailable");
      return api.controlIssueExecution(issueId, action, revision);
    },
    onSettled: async () => {
      await qc.invalidateQueries({ queryKey: issueKeys.all(wsId) });
      await qc.invalidateQueries({ queryKey: issueKeys.tasks(issueId) });
      await qc.invalidateQueries({ queryKey: issueKeys.timeline(issueId) });
    },
  });
}

export function useSaveExecutionPlan(wsId: string, issueId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: import("./execution-plan").ExecutionPlanInput) =>
      api.saveExecutionPlan(issueId, input),
    onSettled: () => qc.invalidateQueries({ queryKey: issueKeys.all(wsId) }),
  });
}

export function useSaveDeliveryEvidence(wsId: string, issueId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ revision, evidence }: { revision: number; evidence: import("./delivery").DeliveryEvidenceInput }) =>
      api.saveDeliveryEvidence(issueId, revision, evidence),
    onSettled: () => qc.invalidateQueries({ queryKey: issueKeys.all(wsId) }),
  });
}

export function deliveryReviewOptions(wsId: string, issueId: string, revision?: number) {
  return queryOptions({
    queryKey: [...issueKeys.all(wsId), "delivery-review", issueId, revision],
    queryFn: () => api.getDeliveryReview(issueId),
  });
}

export function releaseReadinessOptions(wsId: string, issueId: string, revision?: number) {
  return queryOptions({
    queryKey: [...issueKeys.all(wsId), "release-readiness", issueId, revision],
    queryFn: () => api.getReleaseReadiness(issueId),
    refetchInterval: 15_000,
  });
}

export function useReviewDelivery(wsId: string, issueId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: import("./delivery").DeliveryReviewInput) => api.reviewDelivery(issueId, input),
    onSettled: () => qc.invalidateQueries({ queryKey: issueKeys.all(wsId) }),
  });
}

export function useChangeIssueRequirements(wsId: string, issueId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      action,
      revision,
      reason,
    }: {
      action: "start" | "reconcile" | "edit" | "cancel" | "resume";
      revision: number;
      reason?: string;
    }) => api.changeIssueRequirements(issueId, action, revision, reason),
    onSettled: async () => {
      await qc.invalidateQueries({ queryKey: issueKeys.all(wsId) });
      await qc.invalidateQueries({ queryKey: issueKeys.tasks(issueId) });
      await qc.invalidateQueries({ queryKey: issueKeys.timeline(issueId) });
    },
  });
}

export function businessTemplatesOptions(wsId: string) {
  return queryOptions({
    queryKey: ["workbench", wsId, "business-templates"],
    queryFn: () => api.listBusinessTemplates(),
  });
}

export function projectTemplateOptions(wsId: string, projectId: string) {
  return queryOptions({
    queryKey: ["projects", wsId, "detail", projectId, "business-template"],
    queryFn: () => api.getProjectBusinessTemplate(projectId),
  });
}

export function usePublishBusinessTemplate(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ content, id, revision }: { content: BusinessTemplate; id?: string; revision?: number }) => api.saveBusinessTemplate(content, id, revision),
    onSuccess: () => qc.invalidateQueries({ queryKey: businessTemplatesOptions(wsId).queryKey }),
  });
}

export function useSaveProjectTemplate(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ content, revision, mergeSourceRevision }: ProjectTemplate & { mergeSourceRevision?: number }) =>
      mergeSourceRevision === undefined
        ? api.saveProjectBusinessTemplate(projectId, content, revision)
        : api.saveProjectBusinessTemplate(projectId, content, revision, mergeSourceRevision),
    onSuccess: (data) => {
      qc.setQueryData(projectTemplateOptions(wsId, projectId).queryKey, data);
    },
  });
}

export const workbenchKeys = {
  roles: (wsId: string, projectId: string) =>
    ["projects", wsId, "detail", projectId, "workbench-roles"] as const,
  targets: (wsId: string, projectId: string) =>
    ["workbench", wsId, "targets", projectId] as const,
  credentials: (wsId: string) => ["workbench", wsId, "credentials"] as const,
};

export function projectRolesOptions(wsId: string, projectId: string) {
  return queryOptions({
    queryKey: workbenchKeys.roles(wsId, projectId),
    queryFn: () => api.getProjectWorkbenchRoles(projectId),
  });
}

export function useSaveProjectRoles(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ config, revision }: ProjectRoles) =>
      api.saveProjectWorkbenchRoles(projectId, config, revision),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: workbenchKeys.roles(wsId, projectId) }),
  });
}
export function deployTargetsOptions(wsId: string, projectId: string) {
  return queryOptions({
    queryKey: workbenchKeys.targets(wsId, projectId),
    queryFn: () => api.listDeployTargets(projectId),
    refetchInterval: (query) => query.state.data?.targets.some((target) => ["queued", "running"].includes(target.last_check?.status ?? "")) ? 5000 : 60000,
  });
}

export function deploymentCheckStatus(target: DeployTarget, now = Date.now()) {
  if (target.missing_fields.length) return "incomplete";
  const check = target.last_check;
  if (!check) return "untested";
  if (check.status !== "passed") return check.status;
  const age = now - Date.parse(check.at);
  if (!check.ok || !check.free_bytes || !Number.isFinite(age) || age < 0) return "failed";
  return age < 24 * 60 * 60 * 1000 ? "passed" : "expired";
}

export function useCheckDeployTarget(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, revision }: { id: string; revision: number }) => api.checkDeployTarget(projectId, id, revision),
    onSuccess: () => qc.invalidateQueries({ queryKey: workbenchKeys.targets(wsId, projectId) }),
  });
}
export function credentialsOptions(wsId: string) {
  return queryOptions({
    queryKey: workbenchKeys.credentials(wsId),
    queryFn: () => api.listWorkbenchCredentials(),
  });
}
export function credentialReferencesOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: [...workbenchKeys.credentials(wsId), id, "references"],
    queryFn: () => api.listWorkbenchCredentialReferences(id),
    enabled: !!id,
  });
}
export function useSaveDeployTarget(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id?: string; data: DeployTargetInput }) =>
      api.saveDeployTarget(projectId, data, id),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: workbenchKeys.targets(wsId, projectId),
      }),
  });
}
export function useDeleteDeployTarget(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, revision }: { id: string; revision: number }) =>
      api.deleteDeployTarget(projectId, id, revision),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: workbenchKeys.targets(wsId, projectId),
      }),
  });
}
export function useSaveWorkbenchCredential(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      id,
      data,
    }: {
      id?: string;
      data: CredentialInput;
    }) => {
      try {
        return await api.saveWorkbenchCredential(data, id);
      } finally {
        data.secret = "";
      }
    },
    onSuccess: () => Promise.all([
      qc.invalidateQueries({ queryKey: workbenchKeys.credentials(wsId) }),
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) }),
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() }),
    ]),
    gcTime: 0,
  });
}
export function useDeleteWorkbenchCredential(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, revision }: { id: string; revision: number }) =>
      api.deleteWorkbenchCredential(id, revision),
    onSuccess: () => Promise.all([
      qc.invalidateQueries({ queryKey: workbenchKeys.credentials(wsId) }),
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) }),
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() }),
    ]),
  });
}

export { templateMergeFields, mergeBusinessTemplateFields, type TemplateMergeField } from "./template-merge";
export { initializeDefaultTeam } from "./default-team";

export * from "./onespec";
export { currentExecutionIssues, taskWorkspaceProgress, conversationCommand, conversationDestination, canControlWorkspaceExecution } from "./task-workspace";
