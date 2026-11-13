import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";

test.use({ locale: "en-US" });

test("keeps new empty requirements out of the editor and finds the task after project assignment", async ({ page }) => {
  const api = await createTestApi();
  try {
    const project = await api.createProject("New task project assignment");
    const issue = await api.createIssue("Configure a remote CLI", { description: "Use the designated remote server." });
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/issues/${issue.id}`);
    await expect(page.getByTestId("comment-composer-shell")).toHaveCount(1);
    await expect(page.getByRole("button", { name: "Add criterion", exact: true })).toHaveCount(0);
    await expect(page.locator("textarea")).toHaveCount(0);
    await page.getByTestId("comment-composer-shell").click();
    const editor = page.getByRole("textbox", { name: "Comment or instruction for this task", exact: true });
    await editor.fill("Install it only on the remote server.");
    const saved = page.waitForResponse(response => response.request().method() === "POST" && response.url().endsWith(`/issues/${issue.id}/comments`));
    await page.getByRole("button", { name: "Post comment", exact: true }).click();
    expect((await saved).status()).toBe(201);
    const current = await api.getIssue(issue.id);
    await api.updateIssue(issue.id, { project_id: project.id, expected_revision: current.revision });
    await expect(page.getByTestId("task-workspace")).toBeVisible();
    await expect(page.getByTestId("comment-composer-shell")).toHaveCount(1);
    await expect(page.getByText("Install it only on the remote server.", { exact: true })).toBeVisible();
    await expect(page.getByTestId("requirements-summary")).toHaveCount(0);
    await expect(page.locator("textarea")).toHaveCount(0);
    await page.reload();
    await expect(page.getByTestId("task-workspace")).toBeVisible();
    await page.goto(`/${slug}/projects/${project.id}`);
    await expect(page.getByText(issue.title, { exact: true })).toBeVisible();
  } finally {
    await api.cleanup();
  }
});
