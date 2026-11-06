// @vitest-environment node
import { expect, it } from "vitest";
import type { IssueCreateManual } from "../issues/stores/draft-store";
import type { ProjectTemplate } from "./schemas";
import { prefillTaskTemplate } from "./template-prefill";

const draft: IssueCreateManual = {
  title: "Task",
  description: "",
  status: "todo",
  startDate: null,
  labelIds: [],
  propertyValues: {},
};
const template: ProjectTemplate = {
  source_id: "blank",
  source_revision: 1,
  revision: 2,
  content: {
    v: 1,
    name: "Template",
    background: ["Context", "Users", "Current state", "Goal"],
    audience: [],
    scope_out: ["Leave billing unchanged"],
    ac_skeleton: ["Deliver {feature}"],
    forbidden: [],
    dod: [],
    resources: [],
    terms: {},
  },
};

it("prefills separately editable criteria and boundaries, preserving edits across project changes", () => {
  const first = { ...draft, ...prefillTaskTemplate(draft, "one", template) };
  expect(first.requirementsText).toBe("Deliver {feature}");
  expect(first.requirementsBoundsText).toBe("Leave billing unchanged");
  expect(prefillTaskTemplate(first, "one", template)).toBeUndefined();
  const edited = { ...first, requirementsText: "User acceptance" };
  const second = {
    ...edited,
    ...prefillTaskTemplate(edited, "two", {
      ...template,
      content: {
        ...template.content,
        background: [],
        scope_out: ["New boundary"],
        ac_skeleton: ["Different acceptance"],
      },
    }),
  };
  expect(second.requirementsText).toBe("User acceptance");
  expect(second.requirementsBoundsText).toBe("New boundary");
  expect(second.description).toBe("");
  const cleared = { ...second, ...prefillTaskTemplate(second, "", null) };
  expect(cleared.requirementsText).toBe("User acceptance");
  expect(cleared.requirementsBoundsText).toBe("");
});

it("preserves a restored draft and does not overwrite unsaved editor input", () => {
  const patch = prefillTaskTemplate(
    {
      ...draft,
      description: "Typing before the query returned",
      requirementsText: "Own requirement",
    },
    "one",
    template,
  );
  expect(patch).not.toHaveProperty("description");
  expect(patch).not.toHaveProperty("requirementsText");
  const restored = { ...draft, ...patch, requirementsText: "" };
  expect(
    prefillTaskTemplate(restored, "one", { ...template, revision: 3 }),
  ).toBeUndefined();
});
