import { z } from "zod";
import { ProjectRolesSchema } from "./schemas";

export const OneSpecConfigurationSchema = z.object({
  runtime_source: z.enum(["workspace", "project"]).optional().catch(undefined),
  setup: z.object({
    status: z.string(),
    roles: z.array(z.object({
      key: z.string(),
      name: z.string(),
      enabled: z.boolean(),
      agent_id: z.string().nullable(),
      runtime_id: z.string().nullable(),
      status: z.string(),
      binding_source: z.string(),
    })),
    team: ProjectRolesSchema,
  }).optional(),
  enabled: z.boolean().default(false),
  default_runtime_id: z.string().nullable().default(null),
  runtime_id: z.string().nullable().default(null),
  effective_runtime_id: z.string().nullable().default(null),
  overview_repository_id: z.string().nullable().default(null),
  revision: z.number().int().nonnegative(),
});
const OneSpecTaskSchema = z.object({
  key: z.string(),
  title: z.string(),
  description: z.string(),
  role: z.string(),
  repo_id: z.string(),
  covers: z.array(z.string()),
  depends_on: z.array(z.string()),
  existing_dependencies: z.array(z.string()).optional(),
  serial_group: z.string(),
  expected: z.array(z.string()),
});
const DocumentFileSchema = z.object({
  ref: z.object({ repository_id: z.string(), path: z.string(), commit: z.string(), digest: z.string() }),
  content: z.string(), verified: z.boolean().default(false), error: z.string().default(""),
});
export const RequirementsDocumentSchema = DocumentFileSchema.extend({
  prototype_status: z.string().optional(),
  prototype: DocumentFileSchema.nullish(),
});
export const OneSpecArtifactSchema = z.object({
  document: RequirementsDocumentSchema.nullish(),
  draft_revision: z.number().int().nonnegative().optional().catch(0),
  delivery_digest: z.string().optional(),
  release_revision: z.number().optional(),
  stage: z.string(),
  version: z.number().int().positive(),
  summary: z.string(),
  refs: z.array(
    z.object({
      repository_id: z.string(),
      path: z.string(),
      commit: z.string(),
      digest: z.string(),
    }),
  ),
  tasks: z.array(OneSpecTaskSchema).default([]),
  requirement_version: z.number().int(),
  submitted_by: z.string().default(""),
  created_at: z.string().default(""),
});
export const OneSpecResponseSchema = z.object({
  revision: z.number().int().nonnegative(),
  can_confirm: z.boolean().default(false),
  flow: z
    .object({
      generation: z.number().default(0),
      release_id: z.string().optional(),
      v: z.literal(1),
      kind: z.string(),
      phase: z.string(),
      status: z.string(),
      runtime_id: z.string(),
      overview_repository_id: z.string(),
      approver_id: z.string(),
      rule_version: z.string(),
      team: ProjectRolesSchema,
      artifacts: z.array(OneSpecArtifactSchema),
      decisions: z.array(
        z.object({
          stage: z.string(),
          version: z.number(),
          approved: z.boolean(),
          reason: z.string(),
          actor_id: z.string(),
          created_at: z.string(),
        }),
      ),
      git_operations: z.array(
        z.object({
          key: z.string(),
          repository_id: z.string(),
          kind: z.string(),
          branch: z.string(),
          base_commit: z.string(),
          worktree: z.string().optional().default(""),
          published_commit: z.string().optional(),
          merged_commit: z.string().optional(),
          status: z.string(),
          summary: z.string(),
        }),
      ),
    })
    .nullable(),
});
export type OneSpecConfiguration = z.infer<typeof OneSpecConfigurationSchema>;
export type OneSpecResponse = z.infer<typeof OneSpecResponseSchema>;
export const oneSpecKinds = [
  "feature",
  "change",
  "bugfix",
  "release",
  "documentation",
] as const;
export function oneSpecStages(kind: string) {
  switch (kind) {
    case "feature":
      return [
        "requirements",
        "design",
        "plan",
        "branch",
        "implementation",
        "testing",
        "archive",
        "archiving",
        "acceptance",
      ];
    case "change":
    case "bugfix":
      return [
        "requirements",
        "plan",
        "branch",
        "implementation",
        "testing",
        "archive",
        "archiving",
        "acceptance",
      ];
    case "release":
      return [
        "requirements",
        "release_preparation",
        "deployment",
        "merge",
        "merging",
        "acceptance",
      ];
    case "documentation":
      return [
        "requirements",
        "branch",
        "documentation",
        "testing",
        "merge",
        "merging",
        "acceptance",
      ];
    default:
      return [];
  }
}
