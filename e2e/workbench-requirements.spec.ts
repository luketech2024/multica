import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench requirements", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;

  test.afterEach(async () => { await api?.cleanup(); });

  test("edits, confirms and retains its own requirements after reload", async ({ page }) => {
    api = await createTestApi();
    const issue = await api.createIssue(`Workbench requirements ${Date.now()}`, {
      acceptance_criteria: {
        v: 1,
        items: [{ text: "Export the current project report" }],
        bounds: ["No billing changes"],
      },
    });
    expect(issue.id).toBeTruthy();
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    await page.goto(`/${slug}/issues/${issue.identifier}`);
    const text = page.getByRole("textbox", { name: "Criterion A-01" });
    await expect(text).toHaveValue("Export the current project report");
    await text.fill("Export a CSV report for the current project");
    await page.getByRole("button", { name: "Save changes", exact: true }).click();
    const check = page.getByRole("checkbox", { name: "Check A-01" });
    await expect(check).toBeEnabled();
    await check.click();
    await expect(check).toBeChecked();
    const confirm = page.getByRole("button", { name: "Confirm requirements", exact: true });
    await expect(confirm).toBeEnabled();
    await confirm.click();
    await expect(page.getByText("Requirements confirmed and locked")).toBeVisible();
    await page.reload();
    await expect(text).toHaveValue("Export a CSV report for the current project");
    await expect(text).toHaveAttribute("readonly");
    await expect(check).toBeChecked();
    await expect(check).toBeDisabled();
    await expect(page.getByText("Requirements confirmed and locked")).toBeVisible();
    expect(errors).toEqual([]);
  });
});
