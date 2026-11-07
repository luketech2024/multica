import { z } from "zod";
import { parseWithFallback } from "../api/schema";

export const projectRoleOrder = [
  "sched",
  "product",
  "dev",
  "test",
  "sec",
  "ops",
] as const;
export type ProjectRoleKey = (typeof projectRoleOrder)[number];
const ProjectRoleSchema = z.object({
  enabled: z.boolean(),
  agent_id: z.string().nullable().default(null),
  runtime_id: z.string().nullable().default(null),
});
export const ProjectRolesSchema = z.object({
  config: z.object({
    v: z.literal(1),
    roles: z.object({
      sched: ProjectRoleSchema,
      product: ProjectRoleSchema,
      dev: ProjectRoleSchema,
      test: ProjectRoleSchema,
      sec: ProjectRoleSchema,
      ops: ProjectRoleSchema,
    }),
  }),
  revision: z.number().int().positive(),
});
export type ProjectRoles = z.infer<typeof ProjectRolesSchema>;

export const BusinessTemplateSchema = z.object({
  v: z.literal(1),
  name: z.string().min(1).refine((value) => Array.from(value).length <= 80),
  background: z.array(z.string()),
  audience: z.array(z.string()),
  scope_out: z.array(z.string()),
  ac_skeleton: z.array(z.string().refine((value) => Array.from(value).length <= 120)),
  forbidden: z.array(
    z.object({ rule: z.string().min(1), check: z.string().min(1) }),
  ),
  dod: z.array(z.string()),
  resources: z.array(
    z.object({
      type: z.string(),
      ref: z.string().min(1),
      version: z.string().min(1),
    }),
  ),
  terms: z.record(z.string(), z.string()),
});
export const BusinessTemplateAssetSchema = z.object({
  id: z.string(),
  revision: z.number().int().positive(),
  content: BusinessTemplateSchema,
  disabled_roles: z.array(z.enum(projectRoleOrder)),
});
export const BusinessTemplatesSchema = z.object({
  templates: z.array(BusinessTemplateAssetSchema),
});
export const ProjectTemplateSchema = z.object({
  source_id: z.string(),
  source_revision: z.number().int().nonnegative(),
  revision: z.number().int().positive(),
  content: BusinessTemplateSchema,
});
export const ProjectTemplateResponseSchema = z.object({
  template: ProjectTemplateSchema.nullable(),
});
export type BusinessTemplate = z.infer<typeof BusinessTemplateSchema>;
export type BusinessTemplateAsset = z.infer<typeof BusinessTemplateAssetSchema>;
export type ProjectTemplate = z.infer<typeof ProjectTemplateSchema>;

const RepositoryCommandsSchema = z.object({
  install: z.string(), test: z.string(), lint: z.array(z.string()), scan: z.string(),
});
export const RepositoryStackCatalogSchema = z.object({
  templates: z.array(z.object({id: z.string(), family: z.string(), runtime: z.string(), derived: RepositoryCommandsSchema, requires: z.array(z.string()).default([])})),
  tags: z.array(z.object({id: z.string()})),
  deliveries: z.array(z.string()),
});
export const RepositoryProfileSchema = z.object({
  revision: z.number().int().nonnegative(),
  stack: z.object({
    v: z.literal(3), template: z.string(), tags: z.array(z.string()),
    detected: z.record(z.string(), z.unknown()).nullable(),
    baseline: z.record(z.string(), z.unknown()).nullable(),
    derived: RepositoryCommandsSchema,
    overrides: z.record(z.string(), z.object({value:z.string(),by:z.literal("manual")})),
    runtime: z.string(), requires: z.array(z.string()), checks: z.array(z.string()),
  }),
  delivery: z.array(z.string()).min(1),
});
export type RepositoryProfile = z.infer<typeof RepositoryProfileSchema>;
export interface RepositoryProfileInput {
  expected_revision: number;
  template: string;
  tags: string[];
  delivery: string[];
  overrides: Record<string, string>;
}

export const DeployTargetSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  project_id: z.string(),
  repo_id: z.string(),
  label: z.string(),
  env: z.string(),
  kind: z.string(),
  channel: z.string(),
  runtime_id: z.string().nullable(),
  credential_id: z.string().nullable(),
  location: z.record(z.string(), z.string()),
  revision: z.number().int().positive(),
  missing_fields: z.array(z.string()),
  last_check: z
    .object({
      status: z.enum(["queued", "running", "passed", "failed", "expired"]).catch("failed").default("failed"),
      at: z.string(),
      ok: z.boolean(),
      ms: z.number(),
      detail: z.string(),
      free_bytes: z.number().int().nonnegative().nullable().catch(null).default(null),
    })
    .nullable()
    .default(null),
});
export const CredentialSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  project_id: z.string().nullable(),
  label: z.string(),
  kind: z.string(),
  usage: z.array(z.string()),
  last_four: z.string(),
  expires_at: z.string().nullable(),
  rotated_at: z.string(),
  revision: z.number().int().positive(),
});
export const DeployTargetsSchema = z.object({
  targets: z.array(DeployTargetSchema),
});
export const CredentialsSchema = z.object({
  credentials: z.array(CredentialSchema),
  encryption_configured: z.boolean().default(false),
});
export type DeployTarget = z.infer<typeof DeployTargetSchema>;
export type WorkbenchCredential = z.infer<typeof CredentialSchema>;
export type DeployTargetInput = Pick<
  DeployTarget,
  | "repo_id"
  | "label"
  | "env"
  | "kind"
  | "channel"
  | "runtime_id"
  | "credential_id"
  | "location"
> & { expected_revision?: number };
export interface CredentialInput {
  label: string;
  kind: string;
  project_id: string | null;
  usage: string[];
  secret: string;
  expires_at: string | null;
  expected_revision?: number;
}

export function parseWorkbenchResponse<T>(
  data: unknown,
  schema: z.ZodType<T>,
  endpoint: string,
): T {
  const result = parseWithFallback<T | null>(data, schema, null, { endpoint });
  if (result === null)
    throw new Error("Invalid workbench configuration response");
  return result;
}

export const targetLocationFields: Record<string, string[]> = {
  "docker-host": ["host", "image", "dir", "domain", "health_url"],
  "service-process": ["host", "dir", "start_command", "domain", "health_url"],
  "weapp-ci": ["appid", "robot"],
  "static-site": ["bucket", "domain"],
  "npm-package": ["registry"],
};

export function deploymentCredentialKind(
  kind: string,
  channel: string,
): string | undefined {
  if (channel === "ssh") return "ssh-key";
  return (
    {
      "weapp-ci": "weapp-ci-key",
      "static-site": "cloud-ak",
      "npm-package": "registry-token",
    } as Record<string, string>
  )[kind];
}
