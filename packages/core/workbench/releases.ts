import {
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { api } from "../api";
import type { ReleaseOrderInput } from "./release-schemas";
export * from "./release-schemas";

export function releaseOrdersOptions(wsId: string, projectId = "", env = "") {
  return infiniteQueryOptions({
    queryKey: ["releases", wsId, "list", projectId, env],
    initialPageParam: "",
    queryFn: ({ pageParam }) =>
      api.listReleaseOrders(projectId, env, pageParam),
    getNextPageParam: (page) => page.next_cursor || undefined,
  });
}

export function releaseOrderOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: ["releases", wsId, "detail", id],
    queryFn: () => api.getReleaseOrder(id),
    refetchInterval: 15_000,
  });
}

export function releaseCandidatesOptions(
  wsId: string,
  projectId: string,
  env: "staging" | "prod",
) {
  return infiniteQueryOptions({
    queryKey: ["releases", wsId, "candidates", projectId, env],
    initialPageParam: "",
    queryFn: ({ pageParam }) =>
      api.listReleaseCandidates(projectId, env, pageParam),
    getNextPageParam: (page) => page.next_cursor || undefined,
    enabled: Boolean(projectId),
  });
}

export function useCreateReleaseOrder(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: ReleaseOrderInput) => api.createReleaseOrder(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["releases", wsId] }),
  });
}
