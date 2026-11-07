import type { BusinessTemplate } from "./schemas";

export const templateMergeFields = ["name", "background", "audience", "scope_out", "ac_skeleton", "forbidden", "dod", "resources", "terms"] as const;
export type TemplateMergeField = (typeof templateMergeFields)[number];

export function mergeBusinessTemplateFields(current: BusinessTemplate, incoming: BusinessTemplate, fields: readonly TemplateMergeField[]): BusinessTemplate {
  return fields.reduce((merged, field) => ({ ...merged, [field]: incoming[field] }), { ...current });
}
