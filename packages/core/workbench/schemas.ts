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
      at: z.string(),
      ok: z.boolean(),
      ms: z.number(),
      detail: z.string(),
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
