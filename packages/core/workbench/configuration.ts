import {
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { api } from "../api";
import { issueKeys } from "../issues/queries";
import type {
  DeployTargetInput,
  CredentialInput,
  ProjectRoles,
  ProjectTemplate,
} from "./schemas";
export * from "./schemas";
export { prefillTaskTemplate } from "./template-prefill";
export type { ExecutionPlanInput, ExecutionPlanTask } from "./execution-plan";
export { parseWorkbenchHandoff, analyzeExecutionPlan } from "./execution-plan";
export type { WorkbenchHandoff } from "./execution-plan";
export type { ExecutionControlAction, ExecutionPause } from "./execution-control";

export function useControlIssueExecution(wsId: string, issueId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ action, revision }: { action: import("./execution-control").ExecutionControlAction; revision: number }) =>
      api.controlIssueExecution(issueId, action, revision),
    onSettled: async () => {
      await qc.invalidateQueries({ queryKey: issueKeys.all(wsId) });
      await qc.invalidateQueries({ queryKey: issueKeys.tasks(issueId) });
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

export function useSaveProjectTemplate(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ content, revision }: ProjectTemplate) =>
      api.saveProjectBusinessTemplate(projectId, content, revision),
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
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: workbenchKeys.credentials(wsId) }),
    gcTime: 0,
  });
}
export function useDeleteWorkbenchCredential(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, revision }: { id: string; revision: number }) =>
      api.deleteWorkbenchCredential(id, revision),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: workbenchKeys.credentials(wsId) }),
  });
}
