import { z } from "zod";

export const ApplicationDeploymentSchema = z.object({
  nginx_config_directory: z.string().optional(),
  kind: z.enum(["nginx", "docker", "process", "kubernetes"]),
  nodes: z.array(z.string().uuid()).default([]),
  directory: z.string().default(""),
  port: z.string().default("8080"),
  domain: z.string().default(""),
  https: z.enum(["off", "existing", "managed"]).default("off"),
  certificate_id: z.string().default(""),
  strategy: z.enum(["serial", "rolling", "batch"]).default("serial"),
  health_path: z.string().default("/"),
  command: z.string().default(""),
  namespace: z.string().default("default"),
  spa: z.boolean().default(false),
});
export const DeploymentApplicationSchema = z.object({
  id: z.string().uuid(),
  name: z.string(),
  source_kind: z.enum(["git", "image", "directory"]),
  source: z.string(),
  repo_id: z.string().default(""),
  artifact_path: z.string().default("."),
  build_command: z.string().default(""),
  environments: z.record(z.string(), ApplicationDeploymentSchema).default({}),
});
export const DeploymentResourceSchema = z.object({
  auth_method: z.enum(["key", "password"]).optional(),
  id: z.string().uuid(),
  name: z.string(),
  env: z.enum(["staging", "prod"]),
  host: z.string(),
  port: z.string().default("22"),
  username: z.string().default("deploy"),
  host_key: z.string().default(""),
  credential_id: z.string().default(""),
});
export const DeploymentHostDiscoverySchema = z.object({
  host_key: z.string().regex(/^SHA256:[A-Za-z0-9+/]{43}$/),
});
export const ProjectDeploymentConfigSchema = z.object({
  mode: z.enum(["auto", "orchestrated"]).default("auto"),
  applications: z.array(DeploymentApplicationSchema).default([]),
  resources: z.array(DeploymentResourceSchema).default([]),
});
export const ProjectDeploymentSchema = z.object({
  config: ProjectDeploymentConfigSchema,
  revision: z.number().int().nonnegative(),
  deployments: z
    .array(
      z.object({
        application_id: z.string().uuid(),
        env: z.string(),
        release_id: z.string().uuid(),
        node_id: z.string().uuid(),
        version: z.string(),
        status: z.string(),
      }),
    )
    .optional(),
});
export const DeploymentConnectionCheckSchema = z.object({
  ok: z.boolean(),
  detail: z.string(),
});
export const DeploymentCertificateSchema = z.object({
  id: z.string().uuid(),
  name: z.string(),
  domains: z.array(z.string()),
  not_before: z.string(),
  not_after: z.string(),
  fingerprint: z.string(),
});
export const DeploymentCertificatesSchema = z.object({
  certificates: z.array(DeploymentCertificateSchema),
  encryption_configured: z.boolean(),
});
export const ReleaseChangeSchema = z.object({
  id: z.string().uuid(),
  type: z.enum(["app", "config", "sql"]),
  name: z.string(),
  application_id: z.string().uuid(),
  version: z.string().default(""),
  resolved_version: z.string().optional(),
  artifact_digest: z.string().optional(),
  after: z.string().default(""),
  body: z.string().default(""),
  path: z.string().default(""),
  database: z.string().default(""),
  rehearsed: z.boolean().default(false),
});
export const ApplicationReleaseSchema = z.object({
  title: z.string(),
  mode: z.enum(["auto", "orchestrated"]),
  changes: z.array(ReleaseChangeSchema),
  snapshot: ProjectDeploymentConfigSchema.optional(),
  results: z
    .array(
      z.object({
        id: z.string(),
        node_id: z.string(),
        status: z.string(),
        detail: z.string(),
        at: z.string(),
      }),
    )
    .default([]),
});
export type ApplicationDeployment = z.infer<typeof ApplicationDeploymentSchema>;
export type DeploymentApplication = z.infer<typeof DeploymentApplicationSchema>;
export type DeploymentResource = z.infer<typeof DeploymentResourceSchema>;
export type ProjectDeploymentConfig = z.infer<
  typeof ProjectDeploymentConfigSchema
>;
export type ProjectDeployment = z.infer<typeof ProjectDeploymentSchema>;
export type DeploymentCertificate = z.infer<typeof DeploymentCertificateSchema>;
export type ApplicationRelease = z.infer<typeof ApplicationReleaseSchema>;
export type ReleaseChange = z.infer<typeof ReleaseChangeSchema>;
export type CertificateUpload = {
  name: string;
  certificate: string;
  private_key: string;
  domain: string;
};
