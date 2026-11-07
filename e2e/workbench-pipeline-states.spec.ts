import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench pipeline states", () => {
  test.use({ locale: "en-US", viewport: { width: 1280, height: 900 } });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("guides empty planning and retains a draft through repository load failure", async ({ page }) => {
    test.setTimeout(150000);
    api = await createTestApi();
    const project = await api.createProject("Pipeline states", { description: "Review loading and retry behavior." });
    await api.createProjectRepository(project.id, "https://example.test/pipeline-states.git");
    let task = await api.createIssue("Plan export", {
      project_id: project.id,
      acceptance_criteria: { v: 1, items: [{ id: "", text: "Export includes all rows" }], bounds: [] },
    });
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/issues/${task.id}/pipeline?stage=product`);
    await expect(page.getByText("No subtasks yet", { exact: true })).toBeVisible({ timeout: 30000 });
    await page.getByRole("button", { name: "Plan subtasks", exact: true }).click();
    await expect(page.getByText("No tasks ready to plan", { exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "Go to tasks", exact: true })).toHaveAttribute("href", `/${slug}/issues?layout=table`);

    task.acceptance_criteria.items[0].checked = { at: "", by: "" };
    task = await api.updateIssue(task.id, { expected_revision: task.revision, acceptance_criteria: task.acceptance_criteria, confirm_requirements: true });
    let releasePending: () => void = () => {};
    const pending = new Promise<void>((resolve) => { releasePending = resolve; });
    let offline = true;
    await page.route(`**/api/projects/${project.id}/resources`, async (route) => {
      if (!offline) return route.continue();
      await pending;
      await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: "Repository fixture offline" }) });
    });
    await page.reload();
    await page.getByRole("button", { name: "Plan subtasks", exact: true }).click();
    await expect(page.getByRole("status", { name: "Loading repositories…", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Add subtask", exact: true }).click();
    await page.getByRole("textbox", { name: "Subtask title", exact: true }).fill("Retain export plan");
    releasePending();
    await expect(page.getByRole("alert").filter({ hasText: "Planning data could not be loaded." })).toBeVisible({ timeout: 30000 });
    await expect(page.getByRole("button", { name: "Save plan", exact: true })).toBeDisabled();
    offline = false;
    await page.getByRole("button", { name: "Retry repositories", exact: true }).click();
    await expect(page.getByRole("alert").filter({ hasText: "Planning data could not be loaded." })).toHaveCount(0);
    await expect(page.getByRole("textbox", { name: "Subtask title", exact: true })).toHaveValue("Retain export plan");
    await expect(page.getByRole("combobox", { name: "Repository", exact: true }).locator("option")).toHaveCount(2);
    await page.screenshot({ path: ".preview/pipeline-plan-retry.png", fullPage: true });
    expect(errors).toEqual([]);
  });
});
