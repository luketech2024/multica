import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench credential reminders", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("opens the masked credential and preserves the expiry after refresh", async ({ page }) => {
    test.setTimeout(90000);
    api = await createTestApi();
    const project = await api.createProject("Credential reminder project");
    const credentialId = await api.seedCredentialExpiryNotice(project.id);
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/inbox?category=pending`);
    const notice = page.getByRole("button", { name: /Staging deployment key/ });
    await expect(notice).toContainText("Credential expires soon", { timeout: 30000 });
    await notice.click();
    await expect(page).toHaveURL(new RegExp(`tab=credentials&credential=${credentialId}`), { timeout: 30000 });
    await expect(page.getByText("••••1234", { exact: true })).toBeVisible();
    await expect(page.getByText(/^Expires on /)).toBeVisible();
    await page.reload();
    await expect(page.getByText("••••1234", { exact: true })).toBeVisible();
    await expect(page.getByText(/^Expires on /)).toBeVisible();
    await expect(page.getByText("test-encrypted-value", { exact: true })).toHaveCount(0);
    expect(errors).toEqual([]);
  });
});
