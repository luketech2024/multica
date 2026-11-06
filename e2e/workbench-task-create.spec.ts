import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault, preferManualCreateMode } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench task creation", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  let createdId: string | undefined;
  test.afterEach(async () => {
    if (createdId) await api.deleteIssue(createdId);
    await api?.cleanup();
  });

  test("empty project requirements retain their owner and confirmation state after reload", async ({ page }) => {
    api = await createTestApi();
    const project = await api.createProject("Workbench assignment review");
    const slug = await loginAsDefault(page);
    await preferManualCreateMode(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.getByRole("button", { name: "New Issue" }).first().click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox", { name: "Issue title" }).fill("Clarify project acceptance criteria");
    await dialog.getByRole("button", { name: "No project", exact: true }).click();
    await page.getByRole("button", { name: project.title, exact: true }).click();
    const responsePromise = page.waitForResponse((response) =>
      response.request().method() === "POST" && new URL(response.url()).pathname === "/api/issues",
    );
    await dialog.getByRole("button", { name: "Create Issue", exact: true }).click();
    const response = await responsePromise;
    expect(response.status()).toBe(201);
    const created = await response.json();
    createdId = created.id;
    expect(created.project_id).toBe(project.id);
    expect(created.assignee_type).toBe("member");
    expect(created.assignee_id).toBe(created.creator_id);
    expect(created.acceptance_criteria.items).toEqual([]);
    expect(created.acceptance_criteria.gate.spec).toBeNull();
    await page.goto(`/${slug}/issues/${created.id}`);
    const reloaded = page.waitForResponse((result) =>
      result.request().method() === "GET" && new URL(result.url()).pathname === `/api/issues/${created.id}`,
    );
    await page.reload();
    const persisted = await (await reloaded).json();
    expect(persisted.assignee_id).toBe(created.assignee_id);
    expect(persisted.acceptance_criteria).toEqual(created.acceptance_criteria);
    const pipeline = page.getByRole("region", { name: "Pipeline", exact: true });
    await expect(pipeline.getByText("A confirmed execution plan is needed for this requirement version.")).toBeVisible();
    await expect(pipeline.locator("ol > li")).toHaveCount(6);
    await page.screenshot({ path: ".preview/default-assignment-pipeline.png", fullPage: true });
    expect(errors).toEqual([]);
  });
});
