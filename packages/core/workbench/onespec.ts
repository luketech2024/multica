import {
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { api } from "../api";
import { issueKeys } from "../issues/queries";
import type { OneSpecConfiguration } from "./onespec-schemas";
export * from "./onespec-schemas";
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
        queryKey: ["onespec", wsId, "configuration"],
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
export function useProvisionOneSpecTeam(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.provisionOneSpecTeam(projectId),
    onSuccess: async () => {
      await qc.invalidateQueries({
        predicate: (query) => query.queryKey.includes(wsId),
      });
    },
  });
}
