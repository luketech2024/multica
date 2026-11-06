import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench delivery evidence", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("records reports, restores them after reload and marks changed tasks stale", async ({ page }) => {
    test.setTimeout(150000);
    api = await createTestApi();
    const roles = Object.fromEntries(["sched", "product", "dev", "test", "sec", "ops"].map((role) => [role, {
      enabled: ["sched", "product", "test"].includes(role), agent_id: null, runtime_id: null,
    }]));
    const project = await api.createProject("Delivery evidence review", {
      description: "Verify delivery acceptance with a testing-only project team.",
      workbench_roles: { v: 1, roles },
    });
    let parent = await api.createIssue("Review verified export", {
      project_id: project.id,
      acceptance_criteria: { v: 1, items: [{ id: "", text: "Export includes all rows" }], bounds: [] },
    });
    const criteria = parent.acceptance_criteria;
    criteria.items[0].checked = { at: "", by: "" };
    parent = await api.updateIssue(parent.id, { expected_revision: parent.revision, acceptance_criteria: criteria, confirm_requirements: true });
    parent = await api.saveExecutionPlan(parent.id, {
      expected_revision: parent.revision, version: 1, idempotency_key: crypto.randomUUID(),
      tasks: [{ key: "verify", title: "Verify export report", description: "", role: "test", repo_id: "", covers: ["A-01"], depends_on: [], existing_dependencies: [], serial_group: "", expected: ["Export test report"] }],
    });
    const childId = parent.acceptance_criteria.plan.issues[0];
    await api.updateIssue(childId, { status: "done" });
    parent = await api.updateIssue(parent.id, { status: "in_review", expected_revision: parent.revision });
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/issues/${parent.id}`);
    const evidence = page.getByRole("region", { name: "Delivery evidence", exact: true });
    await evidence.getByRole("button", { name: "Record evidence" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox", { name: "Conclusion" }).fill("Export report verified against all expected rows.");
    await dialog.getByRole("textbox", { name: "Export test report" }).fill("https://example.test/reports/export");
    expect(new URL(page.url()).pathname).toBe(`/${slug}/issues/${parent.id}`);
    await dialog.getByRole("checkbox").check();
    const responsePromise = page.waitForResponse((response) => response.request().method() === "POST" && response.url().endsWith("/delivery-evidence"));
    await dialog.getByRole("button", { name: "Save evidence" }).click();
    const response = await responsePromise;
    expect(response.status()).toBe(200);
    const saved = await response.json();
    expect(saved.acceptance_criteria.delivery[childId].version).toBe(1);
    expect(saved.acceptance_criteria.delivery[childId].recorded_by).toBe(parent.creator_id);
    await page.reload();
    await expect(evidence.getByText("Reported as passed", { exact: true })).toBeVisible();
    await expect(evidence.getByRole("link", { name: "Export test report" })).toHaveAttribute("href", "https://example.test/reports/export");
    await page.screenshot({ path: ".preview/delivery-evidence.png", fullPage: true });
    await page.getByRole("link", { name: "Open pipeline", exact: true }).click();
    const stages = page.getByRole("navigation", { name: "Stages", exact: true });
    await expect(stages).toBeVisible({ timeout: 30000 });
    await expect(stages.getByRole("button", { name: /Security/ })).toHaveCount(0);
    await stages.getByRole("button", { name: /Testing/ }).click();
    await expect(page.getByRole("region", { name: "Stage outputs" }).getByText("Verify export report", { exact: true })).toBeVisible();
    await expect(evidence.getByRole("link", { name: "Export test report" })).toHaveAttribute("href", "https://example.test/reports/export");
    await page.reload();
    await expect(stages.getByRole("button", { name: /Testing/ })).toHaveAttribute("aria-current", "step");
    for (const width of [1024, 1280, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await page.screenshot({ path: `.preview/pipeline-${width}.png`, fullPage: true });
    }
    await page.goto(`/${slug}/issues/${parent.id}`);
    const acceptance = page.getByRole("region", { name: "Delivery acceptance", exact: true });
    const acceptedResponse = page.waitForResponse((result) => result.request().method() === "POST" && result.url().endsWith("/delivery-review"));
    await acceptance.getByRole("button", { name: "Accept delivery", exact: true }).click();
    const accepted = await acceptedResponse;
    expect(accepted.status()).toBe(200);
    expect((await accepted.json()).status).toBe("done");
    const undoResponse = page.waitForResponse((result) => result.request().method() === "POST" && result.url().endsWith("/delivery-review"));
    await page.getByRole("button", { name: "Undo", exact: true }).click();
    const undone = await undoResponse;
    expect(undone.status()).toBe(200);
    expect((await undone.json()).status).toBe("in_review");
    await api.updateIssue(childId, { title: "Export verification updated" });
    await page.reload();
    await expect(evidence.getByText("Evidence is out of date", { exact: true })).toBeVisible();
    await expect(acceptance.getByRole("button", { name: "Accept delivery", exact: true })).toBeDisabled();
    await acceptance.getByRole("button", { name: "Request changes", exact: true }).click();
    await acceptance.getByRole("textbox", { name: "Reason for changes" }).fill("Recheck the updated export requirements.");
    const rejectedResponse = page.waitForResponse((result) => result.request().method() === "POST" && result.url().endsWith("/delivery-review"));
    await acceptance.getByRole("button", { name: "Send back for changes" }).click();
    const rejected = await rejectedResponse;
    expect(rejected.status()).toBe(200);
    const changed = await rejected.json();
    expect(changed.status).toBe("in_progress");
    expect(changed.acceptance_criteria.delivery).toBeUndefined();
    await page.reload();
    await expect(evidence.getByText("No delivery evidence recorded.")).toBeVisible();
    expect(errors).toEqual([]);
  });
});
