// @vitest-environment node
import { expect, it } from "vitest";
import { IssueSchema } from "../api/schemas";

it("preserves pause acknowledgements while accepting older issue responses", () => {
  const schema = IssueSchema.pick({ workbench_pause: true });
  const pause = {
    v: 1,
    phase: "unknown",
    previous_status: "in_progress",
    requested_at: "2026-11-07T00:00:00Z",
    requested_by: "11111111-1111-4111-8111-111111111111",
    run_ids: ["22222222-2222-4222-8222-222222222222"],
  };
  expect(schema.parse({ workbench_pause: pause }).workbench_pause).toEqual({
    ...pause,
    pending_runs: [],
    resume_error: "",
  });
  expect(schema.parse({}).workbench_pause).toBeUndefined();
  expect(
    schema.parse({ workbench_pause: { ...pause, phase: "completed" } })
      .workbench_pause,
  ).toBeNull();
  expect(
    schema.parse({ workbench_pause: { ...pause, run_ids: ["invalid"] } })
      .workbench_pause,
  ).toBeNull();
});
