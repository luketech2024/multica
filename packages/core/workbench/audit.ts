import { infiniteQueryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { inboxKeys } from "../inbox/queries";
import type { AuditScope } from "./audit-schemas";
export * from "./audit-schemas";

export function auditOptions(wsId: string, scope: AuditScope, days: number) {
  return infiniteQueryOptions({
    queryKey: ["workbench-audit", wsId, scope, days],
    initialPageParam: "",
    queryFn: ({ pageParam }) => api.listWorkbenchAudit(scope, days, pageParam),
    getNextPageParam: (page) => page.next_cursor || undefined,
  });
}

export function useCreateAuditExport(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ scope, days }: { scope: AuditScope; days: number }) => api.createWorkbenchAuditExport(scope, days),
    onSuccess: () => Promise.all([
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) }),
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() }),
    ]),
  });
}
