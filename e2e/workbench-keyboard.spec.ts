import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault, preferManualCreateMode } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench keyboard flow", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("reads across board columns, respects editing and opens review and help", async ({ page }) => {
    test.setTimeout(120000);
    api = await createTestApi();
    const project = await api.createProject("Keyboard acceptance");
    const older = await api.createIssue("Keyboard older todo", { project_id: project.id });
    const newer = await api.createIssue("Keyboard newer todo", { project_id: project.id });
    const running = await api.createIssue("Keyboard running", { project_id: project.id });
    await api.updateIssue(running.id, { status: "in_progress" });
    const review = await api.createIssue("Keyboard review", { project_id: project.id });
    await api.updateIssue(review.id, { status: "in_review" });
    const slug = await loginAsDefault(page);
    await preferManualCreateMode(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const projectUrl = `/${slug}/projects/${project.id}?wb_mine=0`;
    await page.goto(projectUrl);
    await expect(page.locator('[data-workbench-card="compact"]')).toHaveCount(4);
    const selected = page.locator('a[data-keyboard-selected]');
    for (const issue of [newer, running, review, older]) {
      await page.keyboard.press("j");
      await expect(selected).toContainText(issue.title);
      await expect(selected).toBeFocused();
    }
    await page.keyboard.press("a");
    await expect(page).toHaveURL(new RegExp(`/projects/${project.id}`));
    await page.keyboard.press("k");
    await expect(selected).toContainText(review.title);
    await page.keyboard.press("a");
    await expect(page).toHaveURL(new RegExp(`/issues/${review.identifier}`), { timeout: 30000 });
    await page.keyboard.press("Escape");
    await expect(page).toHaveURL(new RegExp(`/${slug}/issues(?:\\?|$)`));
    await page.goto(projectUrl);
    await expect(page.locator('[data-workbench-card="compact"]')).toHaveCount(4);
    await page.keyboard.press("j");
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/issues/${newer.identifier}`), { timeout: 30000 });
    await page.goto(projectUrl);
    await expect(page.locator('[data-workbench-card="compact"]')).toHaveCount(4);
    await page.keyboard.press("c");
    const create = page.getByRole("dialog");
    await expect(create.getByRole("textbox", { name: "Issue title" })).toBeVisible();
    const title = create.getByRole("textbox", { name: "Issue title" });
    await title.pressSequentially("jka?c");
    await expect(title).toHaveText("jka?c");
    await expect(page.getByRole("dialog", { name: "Keyboard shortcuts" })).toHaveCount(0);
    await page.keyboard.press("Escape");
    await expect(create).toHaveCount(0);
    await expect(page.locator("[data-base-ui-inert]")).toHaveCount(0);
    await page.keyboard.press("Shift+?");
    await expect(page.getByRole("dialog", { name: "Keyboard shortcuts" })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog", { name: "Keyboard shortcuts" })).toHaveCount(0);
    expect(errors).toEqual([]);
  });
});
