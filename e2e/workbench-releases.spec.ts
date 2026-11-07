import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench release orders", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("loads the release center and keeps unaccepted tasks out of each environment", async ({ page }) => {
    test.setTimeout(120000);
    api = await createTestApi();
    const project = await api.createProject("Release eligibility browser check");
    await api.createIssue("Unaccepted release task", { project_id: project.id });
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/releases?project=${project.id}`);
    await expect(page.getByRole("heading", { name: "Releases", exact: true })).toBeVisible();
    await expect(page.getByText("No release orders yet.")).toBeVisible();
    await page.getByRole("button", { name: "New release", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("combobox", { name: "Project", exact: true })).toContainText(project.title);
    await expect(dialog.getByText("No accepted, merged tasks are available for this environment.")).toBeVisible();
    await expect(dialog.getByRole("button", { name: "New release", exact: true })).toBeDisabled();
    await dialog.getByRole("combobox", { name: "Environment", exact: true }).click();
    await page.getByRole("option", { name: "Production", exact: true }).click();
    await expect(page.getByRole("listbox")).toBeHidden();
    await expect(dialog.getByText("No accepted, merged tasks are available for this environment.")).toBeVisible();
    await expect(dialog.getByRole("button", { name: "New release", exact: true })).toBeDisabled();
    await page.screenshot({ path: ".preview/releases-create-empty.png", fullPage: true });
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(dialog).toBeHidden();
    await page.reload();
    await expect(page.getByText("No release orders yet.")).toBeVisible();
    await page.setViewportSize({ width: 1024, height: 720 });
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.screenshot({ path: ".preview/releases-empty.png", fullPage: true });
    expect(errors).toEqual([]);
  });
});
