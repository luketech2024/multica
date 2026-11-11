import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Task interaction P0", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("keeps workflow actions and draft editing reachable across viewport sizes", async ({ page }) => {
    test.setTimeout(120_000);
    api = await createTestApi();
    const project = await api.createProject("P0 interaction check");
    const issue = await api.createIssue("Review the static website", {
      project_id: project.id,
      description: Array.from({ length: 70 }, (_, i) => `Paragraph ${i + 1}. Verify the published website.`).join("\n\n") + "\n\nEND OF DESCRIPTION",
      acceptance_criteria: { v: 1, version: 1, next_item_no: 2,
        items: [{ id: "", text: "The homepage loads", origin: "user", covers: [], checked: null, note: null }],
        bounds: [], gate: { spec: null, accept: null } },
    });
    expect(issue.id, JSON.stringify(issue)).toBeTruthy();
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/issues/${issue.id}`);
    const bar = page.getByRole("region", { name: "Current stage and next step" });
    await expect(bar).toContainText("Confirm requirements");

    for (const size of [{ width: 1440, height: 900 }, { width: 1280, height: 720 }, { width: 1024, height: 768 }]) {
      await page.setViewportSize(size);
      await bar.getByRole("button", { name: "Review requirements", exact: true }).click();
      const confirmation = page.getByRole("button", { name: "Confirm requirements", exact: true });
      await expect(confirmation).toBeInViewport();
      const composer = page.getByTestId("comment-composer-shell");
      const footer = await composer.boundingBox();
      const button = await confirmation.boundingBox();
      expect(button!.y + button!.height).toBeLessThanOrEqual(footer!.y);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    }

    await page.screenshot({ path: ".preview/p0-task-1024.png" });
    // 720 × 450 is the effective CSS viewport of 1440 × 900 at 200% zoom.
    await page.setViewportSize({ width: 720, height: 450 });
    await bar.getByRole("button", { name: "Review requirements", exact: true }).click();
    await page.getByRole("button", { name: "Confirm requirements", exact: true }).scrollIntoViewIfNeeded();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.getByTestId("comment-composer-shell").click();
    const editor = page.getByRole("textbox", { name: "Comment or instruction for this task", exact: true });
    await editor.fill("/note Keep this draft for review");
    await expect(page.getByText("Draft saved", { exact: true })).toBeVisible();
    await page.screenshot({ path: ".preview/p0-composer-1280.png" });
    await editor.press("Escape");
    await expect(page.getByTestId("comment-composer-shell")).toContainText("Continue editing draft");
    await page.reload();
    await expect(editor).toHaveText("/note Keep this draft for review");
    await expect(page.getByText("Draft restored", { exact: true })).toBeVisible();
    await expect(editor).toHaveAttribute("aria-multiline", "true");
    await page.getByRole("button", { name: "Post comment", exact: true }).click();
    await expect(page.getByText("Comment posted.", { exact: true })).toBeVisible();
    await page.reload();
    await expect(page.getByTestId("comment-composer-shell")).toContainText("Add a comment");
    await expect(page.getByTestId("reply-composer-shell")).toHaveCount(0);
    await page.locator(`[id="workflow-${issue.id}-activity"]`).scrollIntoViewIfNeeded();
    await expect(page.locator('[data-message-side="right"]').first()).toBeVisible();
    await page.getByRole("button", { name: "Reply", exact: true }).first().click();
    await expect(page.getByRole("button", { name: "Back to task comment" })).toBeVisible();
    await editor.fill("/note A reply draft");
    await expect(page.getByText("Draft saved", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Back to task comment" }).click();
    await expect(page.getByTestId("comment-composer-shell")).toContainText("Add a comment");
    await page.getByRole("button", { name: "Reply", exact: true }).first().click();
    await expect(editor).toHaveText("/note A reply draft");
    await page.getByRole("button", { name: "Post comment", exact: true }).click();
    await expect(page.locator('[data-message-side="right"]').filter({ hasText: "A reply draft" })).toBeVisible();
    await expect(page.getByTestId("reply-composer-shell")).toHaveCount(0);
    expect(errors).toEqual([]);
  });

  test("chooses an explicit blocked discussion and sends to that original message", async ({ page }) => {
    api = await createTestApi();
    const issue = await api.createIssue("Choose the right discussion", { status: "blocked" });
    const older = await api.createComment(issue.id, "Original deployment question");
    await api.createComment(issue.id, "A newer unrelated discussion");
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/issues/${issue.id}`);
    const bar = page.getByRole("region", { name: "Current stage and next step" });
    const choose = bar.getByRole("button", { name: "Choose discussion and reply", exact: true });
    await choose.click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(page.getByRole("button", { name: "Back to task comment" })).toHaveCount(0);
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(choose).toBeFocused();
    await choose.click();
    await dialog.locator("article").filter({ hasText: "Original deployment question" })
      .getByRole("button", { name: "Reply to this discussion", exact: true }).click();
    await expect(dialog).toBeHidden();
    const editor = page.getByRole("textbox", { name: "Comment or instruction for this task", exact: true });
    await expect(editor).toBeFocused();
    await expect(bar.getByRole("button", { name: "Continue reply", exact: true })).toBeVisible();
    await editor.fill("/note Reply to the original question");
    await expect(page.getByText("Draft saved", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Change discussion", exact: true }).click();
    await dialog.locator("article").filter({ hasText: "A newer unrelated discussion" })
      .getByRole("button", { name: "Reply to this discussion", exact: true }).click();
    await expect(editor).toBeEmpty();
    await page.getByRole("button", { name: "Change discussion", exact: true }).click();
    await dialog.locator("article").filter({ hasText: "Original deployment question" })
      .getByRole("button", { name: "Reply to this discussion", exact: true }).click();
    await expect(editor).toHaveText("/note Reply to the original question");
    const request = page.waitForRequest((req) => req.method() === "POST" && req.url().endsWith(`/issues/${issue.id}/comments`));
    await page.getByRole("button", { name: "Post comment", exact: true }).click();
    expect((await request).postDataJSON().parent_id).toBe(older.id);
    await expect(page.getByText("Comment posted.", { exact: true })).toBeVisible();
    await page.screenshot({ path: ".preview/action-reply-e2e.png" });
  });

});
