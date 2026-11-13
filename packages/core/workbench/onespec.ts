import {
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { api } from "../api";
import { issueKeys } from "../issues/queries";
import { oneSpecStages, type OneSpecConfiguration, type OneSpecResponse } from "./onespec-schemas";
export * from "./onespec-schemas";

export function requirementsHandoff(flow: OneSpecResponse["flow"]) {
  return !!flow && (flow.phase === "requirements" ||
    (flow.phase === oneSpecStages(flow.kind)[1] && flow.status === "preparing"));
}

export async function continueRequirements(issueId: string, version: number) {
  let state = await api.getOneSpecFlow(issueId);
  const artifact = state.flow?.artifacts.findLast((item) => item.stage === "requirements");
  if (!state.flow || !state.can_confirm || artifact?.version !== version || !requirementsHandoff(state.flow)) {
    throw new Error("Requirements changed. Review the current version before continuing.");
  }
  const issue = await api.getIssue(issueId);
  if (issue.workbench_pause) return state;
  if (state.flow.phase === "requirements" && state.flow.status === "awaiting_confirmation") {
    state = await api.updateOneSpecFlow(issueId, "decision", {
      expected_revision: state.revision, stage: "requirements", version, approved: true,
    });
  }
  if (state.flow?.phase === "requirements" && state.flow.status === "ready") {
    if ((await api.getIssue(issueId)).workbench_pause) return state;
    state = await api.updateOneSpecFlow(issueId, "advance", { expected_revision: state.revision });
  }
  if (state.flow && state.flow.phase === oneSpecStages(state.flow.kind)[1] && state.flow.status === "preparing") {
    if ((await api.getIssue(issueId)).workbench_pause) return state;
    state = await api.updateOneSpecFlow(issueId, "run", { expected_revision: state.revision });
  }
  return state;
}

export function useContinueRequirements(wsId: string, issueId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (version: number) => continueRequirements(issueId, version),
    onSettled: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: oneSpecFlowOptions(wsId, issueId).queryKey }),
        qc.invalidateQueries({ queryKey: issueKeys.all(wsId) }),
        qc.invalidateQueries({ queryKey: issueKeys.tasks(issueId) }),
      ]);
    },
  });
}
export function oneSpecConfigurationOptions(wsId: string, projectId?: string) {
  return queryOptions({
    queryKey: ["onespec", wsId, "configuration", projectId ?? "workspace"],
    queryFn: () => api.getOneSpecConfiguration(wsId, projectId),
  });
}
export function oneSpecFlowOptions(wsId: string, issueId: string) {
  return queryOptions({
    queryKey: ["onespec", wsId, "flow", issueId],
    queryFn: () => api.getOneSpecFlow(issueId),
    refetchInterval: 10_000,
  });
}
export function useSaveOneSpecConfiguration(wsId: string, projectId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: OneSpecConfiguration) =>
      api.saveOneSpecConfiguration(wsId, input, projectId),
    onSuccess: async () => {
      await qc.invalidateQueries({
        predicate: (query) => query.queryKey.includes(wsId),
      });
    },
  });
}
export function useOneSpecAction(wsId: string, issueId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      action,
      input,
    }: {
      action: string;
      input: Record<string, unknown>;
    }) => api.updateOneSpecFlow(issueId, action, input),
    onSettled: async () => {
      await Promise.all([
        qc.invalidateQueries({
          queryKey: oneSpecFlowOptions(wsId, issueId).queryKey,
        }),
        qc.invalidateQueries({ queryKey: issueKeys.all(wsId) }),
      ]);
    },
  });
}
export function useProvisionOneSpecTeam(wsId: string, projectId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      if (projectId) await api.provisionOneSpecTeam(projectId);
      else await api.provisionOneSpecWorkspace(wsId);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({
        predicate: (query) => query.queryKey.includes(wsId),
      });
    },
  });
}
