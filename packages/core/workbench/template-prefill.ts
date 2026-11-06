import type { IssueCreateManual } from "../issues/stores/draft-store";
import type { ProjectTemplate } from "./schemas";

export function prefillTaskTemplate(
  draft: IssueCreateManual,
  projectId: string,
  template: ProjectTemplate | null,
): Partial<IssueCreateManual> | undefined {
  if (draft.templatePrefill?.projectId === projectId) return;
  const previous = draft.templatePrefill;
  if (!template && !previous) return;
  const next = {
    projectId,
    revision: template?.revision ?? 0,
    description: template?.content.background.join("\n\n") ?? "",
    requirementsText: template?.content.ac_skeleton.join("\n") ?? "",
    requirementsBoundsText: template?.content.scope_out.join("\n") ?? "",
  };
  const patch: Partial<IssueCreateManual> = { templatePrefill: next };
  for (const field of [
    "description",
    "requirementsText",
    "requirementsBoundsText",
  ] as const) {
    if (!draft[field] || draft[field] === previous?.[field])
      patch[field] = next[field];
  }
  return patch;
}
