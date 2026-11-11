import {
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { api } from "../api";
import type {
  DeploymentResource,
  ProjectDeployment,
  CertificateUpload,
  ApplicationRelease,
} from "./application-deployment-schemas";
export * from "./application-deployment-schemas";
export function useDiscoverDeploymentHost(projectId: string) {
  return useMutation({
    mutationFn: (input: { host: string; port: string }) =>
      api.discoverDeploymentHost(projectId, input),
  });
}
export function projectDeploymentOptions(wsId: string, projectId: string) {
  return queryOptions({
    queryKey: ["project-deployment", wsId, projectId],
    queryFn: () => api.getProjectDeployment(projectId),
    enabled: Boolean(projectId),
    refetchInterval: 15_000,
  });
}
export function deploymentCertificatesOptions(wsId: string, projectId: string) {
  return queryOptions({
    queryKey: ["deployment-certificates", wsId, projectId],
    queryFn: () => api.listDeploymentCertificates(projectId),
    enabled: Boolean(projectId),
  });
}
export function useSaveProjectDeployment(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: ProjectDeployment) =>
      api.saveProjectDeployment(projectId, data),
    onSuccess: (data) => {
      qc.setQueryData(projectDeploymentOptions(wsId, projectId).queryKey, data);
      void qc.invalidateQueries({
        queryKey: ["project-deployment", wsId, projectId],
      });
      void qc.invalidateQueries({ queryKey: ["releases", wsId] });
    },
  });
}
export function useCheckDeploymentResource(
  projectId: string,
  resourceId: string,
) {
  return useMutation({
    mutationFn: () => api.checkDeploymentResource(projectId, resourceId),
  });
}
export function useUploadDeploymentCertificate(
  wsId: string,
  projectId: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CertificateUpload) =>
      api.uploadDeploymentCertificate(projectId, input),
    onSuccess: () =>
      qc.invalidateQueries({
        queryKey: deploymentCertificatesOptions(wsId, projectId).queryKey,
      }),
  });
}
export function useUpdateApplicationRelease(wsId: string, releaseId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: {
      delivery: ApplicationRelease;
      expected_revision: number;
    }) => api.updateApplicationRelease(releaseId, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["releases", wsId] }),
  });
}
export function useExecuteApplicationRelease(wsId: string, releaseId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { expected_revision: number; acknowledged: boolean }) =>
      api.executeApplicationRelease(releaseId, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["releases", wsId] }),
  });
}

export function useCheckDeploymentResourceDraft(projectId: string) {
  return useMutation({
    mutationFn: (resource: DeploymentResource) =>
      api.checkDeploymentResourceDraft(projectId, resource),
  });
}
