import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench requirement inbox", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("keeps a read requirement pending until confirmation", async ({ page }) => {
    test.setTimeout(90000);
    api = await createTestApi();
    const project = await api.createProject("Inbox requirement project");
    let issue = await api.createIssue("Confirm inbox export requirement", {
      project_id: project.id,
      acceptance_criteria: { v: 1, items: [{ id: "", text: "Export includes every row" }], bounds: [] },
    });
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/inbox?category=pending`);
    const row = page.getByRole("button", { name: /Confirm inbox export requirement/ });
    await expect(row).toBeVisible({ timeout: 30000 });
    await expect(row).toContainText("Requirements are ready for your confirmation");
    await row.click();
    await expect(page).toHaveURL(new RegExp(`/issues/${issue.id}$`));
    await page.goto(`/${slug}/inbox?category=pending`);
    await expect(row).toBeVisible();
    const criteria = issue.acceptance_criteria;
    criteria.items[0].checked = { at: "", by: "" };
    issue = await api.updateIssue(issue.id, { expected_revision: issue.revision, acceptance_criteria: criteria, confirm_requirements: true });
    await expect(row).toHaveCount(0, { timeout: 20000 });
    await page.reload();
    await expect(row).toHaveCount(0);
    expect(errors).toEqual([]);
  });
});
