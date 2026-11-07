// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  CredentialSchema,
  CredentialsSchema,
  DeployTargetsSchema,
  DeployTargetSchema,
  ProjectRolesSchema,
  projectRoleOrder,
  ProjectTemplateResponseSchema,
  BusinessTemplateSchema,
  parseWorkbenchResponse,
} from "./schemas";

describe("workbench configuration responses", () => {
  it("fails closed for unknown check states and rejects malformed probe data", () => {
    const check = { status: "future-status", at: "2026-11-07T00:00:00Z", ok: true, ms: 5, detail: "Probe" };
    expect(DeployTargetSchema.shape.last_check.parse(check)?.status).toBe("failed");
    expect(DeployTargetSchema.shape.last_check.safeParse({ ...check, ok: "true" }).success).toBe(false);
    expect(DeployTargetSchema.shape.last_check.safeParse({ ...check, ms: "five" }).success).toBe(false);
  });
  it("counts template limits in Unicode characters", () => {
    const content = { v: 1, name: "😀".repeat(80), background: [], audience: [], scope_out: [], ac_skeleton: ["😀".repeat(120)], forbidden: [], dod: [], resources: [], terms: {} };
    expect(BusinessTemplateSchema.safeParse(content).success).toBe(true);
    expect(BusinessTemplateSchema.safeParse({ ...content, name: content.name + "a" }).success).toBe(false);
  });
  it("rejects project templates with missing prohibition checks", () => {
    const content = { v: 1, name: "Project", background: [], audience: [], scope_out: [], ac_skeleton: [], forbidden: [{ rule: "Do not leak keys", check: "" }], dod: [], resources: [], terms: {} };
    expect(() => parseWorkbenchResponse({ template: { source_id: "blank", source_revision: 1, revision: 1, content } }, ProjectTemplateResponseSchema, "template")).toThrow();
    expect(parseWorkbenchResponse({ template: null }, ProjectTemplateResponseSchema, "template").template).toBeNull();
  });
  it("rejects incomplete roles and invalid revisions while defaulting optional bindings", () => {
    const roles = Object.fromEntries(
      projectRoleOrder.map((role) => [role, { enabled: true }]),
    );
    const input = { config: { v: 1, roles }, revision: 1 };
    expect(
      parseWorkbenchResponse(input, ProjectRolesSchema, "roles").config.roles
        .dev,
    ).toEqual({ enabled: true, agent_id: null, runtime_id: null });
    for (const invalid of [
      { ...input, revision: 0 },
      { ...input, config: { v: 2, roles } },
      { ...input, config: { v: 1, roles: { ...roles, sec: undefined } } },
      {
        ...input,
        config: { v: 1, roles: { ...roles, dev: { enabled: "false" } } },
      },
    ]) {
      expect(() =>
        parseWorkbenchResponse(invalid, ProjectRolesSchema, "roles"),
      ).toThrow();
    }
  });
  it("does not turn a malformed target list into an empty configured list", () => {
    expect(() =>
      parseWorkbenchResponse(
        { targets: [{ id: "target", last_check: { ok: true } }] },
        DeployTargetsSchema,
        "targets",
      ),
    ).toThrow();
    expect(
      parseWorkbenchResponse({ targets: [] }, DeployTargetsSchema, "targets"),
    ).toEqual({ targets: [] });
  });
  it("keeps credential creation disabled when the server omits encryption capability", () => {
    expect(
      parseWorkbenchResponse(
        { credentials: [] },
        CredentialsSchema,
        "credentials",
      ).encryption_configured,
    ).toBe(false);
  });
  it("keeps only masked credential data in the parsed response", () => {
    const input = {
      id: "credential",
      workspace_id: "workspace",
      project_id: null,
      label: "Deployment",
      kind: "token",
      usage: ["deploy"],
      last_four: "1234",
      expires_at: null,
      rotated_at: "2026-11-06T00:00:00Z",
      revision: 1,
      secret: "must-not-cache",
      secret_enc: "ciphertext",
    };
    const parsed = parseWorkbenchResponse(
      input,
      CredentialSchema,
      "credential",
    );
    expect(parsed.last_four).toBe("1234");
    expect(parsed).not.toHaveProperty("secret");
    expect(parsed).not.toHaveProperty("secret_enc");
    expect(() =>
      parseWorkbenchResponse(
        { ...input, revision: "invalid" },
        CredentialSchema,
        "credential",
      ),
    ).toThrow();
  });
});
