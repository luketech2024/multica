// @vitest-environment node
import { expect, it } from "vitest";
import { OneSpecConfigurationSchema } from "./onespec-schemas";
import { ProjectRolesSchema, projectRoleOrder } from "./schemas";

it("accepts older settings without enabling OneSpec", () => {
  expect(OneSpecConfigurationSchema.parse({ revision: 0 })).toMatchObject({
    enabled: false, effective_runtime_id: null,
  });
});

it("preserves automatic bindings and rejects malformed setup responses", () => {
  const team = ProjectRolesSchema.parse({ revision: 1, config: { v: 1,
    roles: Object.fromEntries(projectRoleOrder.map((key) => [key, {
      enabled: true, binding_source: "onespec_default",
    }])),
  } });
  const input = { revision: 0, setup: { status: "pending_runtime", team, roles: [{
    key: "dev", name: "Developer", enabled: true, agent_id: null,
    runtime_id: null, status: "pending_runtime", binding_source: "onespec_default",
  }] } };
  expect(OneSpecConfigurationSchema.parse(input).setup?.team.config.roles.dev.binding_source).toBe("onespec_default");
  expect(OneSpecConfigurationSchema.safeParse({ ...input, setup: { ...input.setup, roles: "invalid" } }).success).toBe(false);
});
