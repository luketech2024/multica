import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench cards", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("uses fixed card heights, responsive columns and paginated task links", async ({ page }) => {
    test.setTimeout(180000);
    api = await createTestApi();
    const project = await api.createProject("Card layout acceptance");
    const title = "A long task title covering multiple repositories and all acceptance criteria";
    const review = await api.createIssue(title, { project_id: project.id });
    await api.updateIssue(review.id, { status: "in_review" });
    for (let index = 0; index < 55; index++) {
      await api.createIssue(`Flow task ${String(index).padStart(2, "0")}`, { project_id: project.id });
    }
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    const tailRows: number[] = [];
    page.on("response", async (response) => {
      if (!response.url().endsWith("/issues/table/rows")) return;
      const request = response.request().postDataJSON();
      if (request.page?.cursor) {
        const result = await response.json().catch(() => ({}));
        if (response.status() === 200 && Array.isArray(result.rows)) tailRows.push(result.rows.length);
      }
    });
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/projects/${project.id}?wb_mine=0`);
    const compact = page.locator('[data-workbench-card="compact"]').filter({ hasText: title });
    await expect(compact).toBeVisible({ timeout: 30000 });
    expect(await compact.evaluate((element) => element.getBoundingClientRect().height)).toBe(72);
    await expect(compact.getByText(title, { exact: true })).toHaveAttribute("title", title);
    await expect(compact.getByText("Review", { exact: true })).toBeVisible();
    await expect(compact.locator("time")).toHaveCount(0);
    await page.getByRole("button", { name: "Display", exact: true }).click();
    await page.getByRole("group", { name: "View", exact: true }).getByRole("button", { name: "List", exact: true }).click();
    await page.keyboard.press("Escape");
    const flow = page.getByRole("region", { name: "Task cards", exact: true });
    await expect(flow).toBeVisible();
    const firstCard = flow.locator('[data-workbench-card="flow"]').first();
    await expect(firstCard).toBeVisible();
    expect(await firstCard.evaluate((element) => element.getBoundingClientRect().height)).toBe(92);
    for (const [width, columns] of [[1024, 2], [1280, 3], [1600, 4]]) {
      await page.setViewportSize({ width, height: 900 });
      await expect.poll(() => flow.locator('[data-testid="virtuoso-item-list"]').evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(columns);
    }
    await page.keyboard.press("k");
    await expect(flow.locator("a[data-keyboard-selected]")).toContainText(title);
    await expect(flow.locator("a[data-keyboard-selected]")).toBeFocused();
    await expect.poll(async () => {
      await flow.locator('[data-virtuoso-scroller]').evaluate((element) => { element.scrollTop = element.scrollHeight; });
      return flow.getByText(title, { exact: true }).count();
    }, { timeout: 30000 }).toBe(1);
    await expect.poll(() => tailRows.reduce((sum, count) => sum + count, 0), { timeout: 15000 }).toBeGreaterThan(0);
    const reviewCard = flow.locator('[data-workbench-card="flow"]').filter({ hasText: title });
    await expect(reviewCard).toBeVisible();
    await expect(reviewCard.getByText(project.title, { exact: false })).toBeVisible();
    await page.screenshot({ path: ".preview/workbench-card-flow.png", fullPage: true });
    await reviewCard.click();
    await expect(page).toHaveURL(new RegExp(`/issues/${review.identifier}`), { timeout: 30000 });
    expect(errors).toEqual([]);
  });
});
