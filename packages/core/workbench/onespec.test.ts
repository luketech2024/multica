// @vitest-environment node
import { beforeEach, expect, it, vi } from "vitest";
import { api } from "../api";
import { continueRequirements } from "./onespec";
import type { OneSpecResponse } from "./onespec-schemas";
import type { Issue } from "../types";

vi.mock("../api", () => ({ api: { getOneSpecFlow: vi.fn(), getIssue: vi.fn(), updateOneSpecFlow: vi.fn() } }));

function state(phase = "requirements", status = "awaiting_confirmation", kind = "feature") {
  return { revision: 3, can_confirm: true, flow: { kind, phase, status,
    artifacts: [{ stage: "requirements", version: 2 }],
  } } as OneSpecResponse;
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.getIssue).mockResolvedValue({ id: "task" } as Issue);
});

it("confirms the reviewed version, advances and starts the returned stage", async () => {
  vi.mocked(api.getOneSpecFlow).mockResolvedValue(state());
  vi.mocked(api.updateOneSpecFlow)
    .mockResolvedValueOnce({ ...state("requirements", "ready"), revision: 4 })
    .mockResolvedValueOnce({ ...state("design", "preparing"), revision: 5 })
    .mockResolvedValueOnce(state("design", "preparing"));
  await continueRequirements("task", 2);
  expect(api.updateOneSpecFlow).toHaveBeenNthCalledWith(1, "task", "decision", { expected_revision: 3, stage: "requirements", version: 2, approved: true });
  expect(api.updateOneSpecFlow).toHaveBeenNthCalledWith(2, "task", "advance", { expected_revision: 4 });
  expect(api.updateOneSpecFlow).toHaveBeenNthCalledWith(3, "task", "run", { expected_revision: 5 });
});

it.each(["change", "bugfix"])("retries only startup after %s requirements have advanced to plan", async (kind) => {
  vi.mocked(api.getOneSpecFlow).mockResolvedValue(state("plan", "preparing", kind));
  vi.mocked(api.updateOneSpecFlow).mockResolvedValue(state("plan", "preparing", kind));
  await continueRequirements("task", 2);
  expect(api.updateOneSpecFlow).toHaveBeenCalledExactlyOnceWith("task", "run", { expected_revision: 3 });
});

it("preserves successful confirmation when advancement fails, and retries advancement only", async () => {
  vi.mocked(api.getOneSpecFlow).mockResolvedValue(state());
  vi.mocked(api.updateOneSpecFlow).mockResolvedValueOnce(state("requirements", "ready")).mockRejectedValueOnce(new Error("offline"));
  await expect(continueRequirements("task", 2)).rejects.toThrow("offline");
  vi.mocked(api.getOneSpecFlow).mockResolvedValue(state("requirements", "ready"));
  vi.mocked(api.updateOneSpecFlow).mockClear().mockResolvedValue(state("design", "preparing"));
  await continueRequirements("task", 2);
  expect(vi.mocked(api.updateOneSpecFlow).mock.calls.map((call) => call[1])).toEqual(["advance", "run"]);
});

it("stops startup when the task is paused after advancement", async () => {
  vi.mocked(api.getOneSpecFlow).mockResolvedValue(state("requirements", "ready"));
  vi.mocked(api.getIssue).mockResolvedValueOnce({ id: "task" } as Issue)
    .mockResolvedValueOnce({ id: "task" } as Issue)
    .mockResolvedValueOnce({ workbench_pause: { phase: "paused" } } as Issue);
  vi.mocked(api.updateOneSpecFlow).mockResolvedValue(state("design", "preparing"));
  await continueRequirements("task", 2);
  expect(api.updateOneSpecFlow).toHaveBeenCalledExactlyOnceWith("task", "advance", { expected_revision: 3 });
});

it("does not confirm or advance a paused task", async () => {
  vi.mocked(api.getOneSpecFlow).mockResolvedValue(state());
  vi.mocked(api.getIssue).mockResolvedValue({ workbench_pause: { phase: "paused" } } as Issue);
  await continueRequirements("task", 2);
  expect(api.updateOneSpecFlow).not.toHaveBeenCalled();
});

it("rejects changed versions, lost permission and a stage outside the handoff", async () => {
  for (const [current, version] of [[state(), 99], [{ ...state(), can_confirm: false }, 2], [state("implementation", "preparing"), 2]] as const) {
    vi.mocked(api.getOneSpecFlow).mockResolvedValue(current);
    await expect(continueRequirements("task", version)).rejects.toThrow();
  }
  expect(api.updateOneSpecFlow).not.toHaveBeenCalled();
});
