import { infiniteQueryOptions, queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import type { SecurityDecision, SecurityStatus, SecurityScanInput } from "./security-schemas";
export * from "./security-schemas";

export function securityChecksOptions(wsId: string, projectId = "", status: SecurityStatus | "" = "") {
  return infiniteQueryOptions({
    queryKey: ["security", wsId, "checks", projectId, status],
    initialPageParam: "",
    queryFn: ({ pageParam }) => api.listSecurityChecks(projectId, status, pageParam),
    getNextPageParam: (page) => page.next_cursor || undefined,
    refetchInterval: 30_000,
  });
}

export function securityCheckOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: ["security", wsId, "check", id],
    queryFn: () => api.getSecurityCheck(id),
    enabled: Boolean(id),
    refetchInterval: 30_000,
  });
}

export function useSecurityDecision(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: SecurityDecision }) => api.updateSecurityCheck(id, input),
    onSettled: () => qc.invalidateQueries({ queryKey: ["security", wsId] }),
  });
}

export function securityScanOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: ["security", wsId, "scan", id],
    queryFn: () => api.getSecurityScan(id),
    enabled: Boolean(id),
  });
}

export function useRecordSecurityScan(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: SecurityScanInput) => api.recordSecurityScan(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["security", wsId] }),
  });
}
