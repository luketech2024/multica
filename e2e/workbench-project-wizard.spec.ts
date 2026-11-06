import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench project wizard", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("Escape closes the repository picker and retains the wizard selection", async ({ page }) => {
    api = await createTestApi();
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/projects`);
    await page.getByRole("button", { name: "New project", exact: true }).first().click();
    const dialog = page.getByRole("dialog");
    await dialog.locator("[contenteditable]").nth(0).fill("Repository keyboard review");
    await dialog.locator("[contenteditable]").nth(1).fill("Keep repository choices when dismissing the picker.");
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await page.getByRole("button", { name: "Repos", exact: true }).click();
    const address = page.getByPlaceholder("https://github.com/owner/repo or git@github.com:owner/repo.git");
    await address.fill("https://example.test/keyboard-review.git");
    await page.getByRole("button", { name: "Add", exact: true }).click();
    await page.keyboard.press("Escape");
    await expect(address).not.toBeVisible();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText("https://example.test/keyboard-review.git", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "1 repo", exact: true })).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(dialog).not.toBeVisible();
  });
});
