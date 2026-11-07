// @vitest-environment node
import { beforeEach, expect, it, vi } from "vitest";
import { api, ApiError } from "../api";
import type { Agent } from "../types";
import { initializeDefaultTeam } from "./default-team";
import { ProjectRolesSchema, projectRoleOrder } from "./schemas";

vi.mock("../api", async (importOriginal) => {
  const original = await importOriginal<typeof import("../api")>();
  return { ...original, api: { listAgents: vi.fn(), createAgent: vi.fn() } };
});
let stored: Agent[];
const config = () => ProjectRolesSchema.parse({ revision: 1, config: { v: 1,
  roles: Object.fromEntries(projectRoleOrder.map(role => [role, {enabled:true}])) } }).config;
const run = (team = config(), onProgress = vi.fn()) => initializeDefaultTeam({
  config:team, runtimeId:"runtime-default", workspaceId:"ws", workspaceSlug:"target-workspace", onProgress,
});
beforeEach(() => {
  vi.resetAllMocks();
  stored = [];
  vi.mocked(api.listAgents).mockImplementation(async () => [...stored]);
  vi.mocked(api.createAgent).mockImplementation(async request => {
    const agent = { ...request, id:`agent-${stored.length}`, archived_at:null } as Agent;
    stored.push(agent);
    return agent;
  });
});
it("creates independent role instructions on one runtime and reuses them on retry", async () => {
  const progress = vi.fn();
  await run(config(), progress);
  expect(stored).toHaveLength(6);
  expect(new Set(stored.map(agent => agent.instructions)).size).toBe(6);
  for (const agent of stored) {
    expect(agent.runtime_id).toBe("runtime-default");
    expect(agent.instructions).toContain("<!-- wb-summary v1 -->");
    expect(agent.instructions).toContain("<!-- /wb-summary -->");
  }
  expect(stored[2]?.max_concurrent_tasks).toBe(2);
  expect(api.createAgent).toHaveBeenCalledWith(expect.any(Object), "target-workspace");
  await run();
  expect(api.createAgent).toHaveBeenCalledTimes(6);
  expect(progress.mock.lastCall?.[0].roles.ops.agent_id).toBe("agent-5");
});
it("preserves manual bindings, disabled roles, and manually selected runtimes", async () => {
  const team = config();
  team.roles.product.agent_id = "manual-agent";
  team.roles.sec.enabled = false;
  team.roles.ops.runtime_id = "another-runtime";
  const progress = vi.fn();
  await run(team, progress);
  expect(stored).toHaveLength(4);
  const final = progress.mock.lastCall?.[0];
  expect(final.roles.product.agent_id).toBe("manual-agent");
  expect(final.roles.sec).toEqual(team.roles.sec);
  expect(stored[3]?.runtime_id).toBe("another-runtime");
  expect(team.roles.sched.agent_id).toBeNull();
});
it("keeps completed roles after failure and resumes without duplicates", async () => {
  const create = vi.mocked(api.createAgent).getMockImplementation()!;
  vi.mocked(api.createAgent).mockImplementationOnce(create).mockRejectedValueOnce(new Error("unavailable"));
  const progress = vi.fn();
  await expect(run(config(), progress)).rejects.toThrow("unavailable");
  expect(progress.mock.lastCall?.[0].roles.sched.agent_id).toBe("agent-0");
  await run();
  expect(stored).toHaveLength(6);
  expect(stored.filter(agent => agent.name.startsWith("调度"))).toHaveLength(1);
});
it("recovers a concurrent creation only when the runtime and instructions match", async () => {
  const create = vi.mocked(api.createAgent).getMockImplementation()!;
  vi.mocked(api.createAgent).mockImplementationOnce(async (request) => {
    await create(request);
    throw new ApiError("exists", 409, "Conflict");
  });
  await run();
  expect(stored).toHaveLength(6);
});
it("does not adopt an unrelated agent with the same name", async () => {
  vi.mocked(api.createAgent).mockRejectedValue(new ApiError("exists", 409, "Conflict"));
  stored.push({name:"调度 · runtime-",runtime_id:"runtime-default",instructions:"unrelated",archived_at:null} as Agent);
  await expect(run()).rejects.toThrow("exists");
});
