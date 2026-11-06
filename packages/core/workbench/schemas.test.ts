// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  CredentialSchema,
  CredentialsSchema,
  DeployTargetsSchema,
  parseWorkbenchResponse,
} from "./schemas";

describe("workbench configuration responses", () => {
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
