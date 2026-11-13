import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";
import type { Page } from "@playwright/test";

async function chooseDestination(page: Page, issueId: string) {
  await page.getByTestId("task-workspace").locator("header").getByRole("button", { name: "Task details", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("combobox", { name: "Task details", exact: true }).selectOption(issueId);
  await dialog.getByRole("button", { name: "Discuss", exact: true }).click();
}

test.describe("Task interaction P0", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("starts the execution task from its parent without navigation", async ({ page }) => {
    api = await createTestApi();
    const project = await api.createProject("Execution guidance");
    let parent = await api.createIssue("Deploy the service", { project_id: project.id,
      acceptance_criteria: { v: 1, version: 1, items: [{ id: "", text: "Service is reachable", origin: "user", covers: [], checked: null, note: null }], bounds: [], gate: { spec: null, accept: null } },
    });
    parent = await api.prepareRequirementsReview(parent.id);
    parent = await api.updateIssue(parent.id, { expected_revision: parent.revision,
      acceptance_criteria: { ...parent.acceptance_criteria, items: parent.acceptance_criteria.items.map((item: Record<string, unknown>) => ({ ...item, checked: { at: "", by: "" } })) },
      confirm_requirements: true,
    });
    parent = await api.saveExecutionPlan(parent.id, { expected_revision: parent.revision, version: parent.acceptance_criteria.version, idempotency_key: "execution-guidance",
      tasks: [{ key: "install", title: "Install the service", description: "Install and verify service", role: "ops", repo_id: "", covers: parent.acceptance_criteria.items.map((item: { id: string }) => item.id), depends_on: [], serial_group: "", expected: ["Service is reachable"] }],
    });
    const childId = parent.acceptance_criteria.plan.issues[0];
    const slug = await loginAsDefault(page);
    await page.route(`**/api/issues/${parent.id}/children`, async (route) => {
      const response = await route.fetch();
      expect(response.ok(), await response.text()).toBe(true);
      const body = await response.json();
      body.issues = body.issues.map((child: Record<string, unknown>) => ({ ...child, assignee_type: "agent", assignee_id: "00000000-0000-4000-8000-000000000001" }));
      await route.fulfill({ response, json: body });
    });
    // Intercept dispatch so this UI test never invokes an installed agent.
    await page.route(`**/api/issues/${childId}/rerun`, (route) => route.fulfill({ json: { id: "fake-run", issue_id: childId, status: "queued" } }));
    await page.goto(`/${slug}/issues/${parent.id}`);
    await expect(page.getByTestId("task-workspace")).toBeVisible();
    await expect(page.getByRole("button", { name: "Review", exact: true })).toHaveCount(0);
    const request = page.waitForRequest((req) => req.method() === "POST" && req.url().endsWith(`/issues/${childId}/rerun`));
    await page.getByRole("button", { name: "Start execution", exact: true }).click();
    expect((await request).postDataJSON()).toEqual({});
    await expect(page).toHaveURL(new RegExp(parent.id));
    await expect(page.getByTestId("conversation-recipient")).toBeVisible();

  });

  test("projects show one parent entry with subtask progress and keep children accessible", async ({ page }) => {
    api = await createTestApi();
    const project = await api.createProject("Parent task overview");
    const parent = await api.createIssue("Deploy the test service", { project_id: project.id });
    const child = await api.createIssue("Install service on the host", { project_id: project.id, parent_issue_id: parent.id });
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/projects/${project.id}`);
    const cards = page.locator('[data-workbench-card="compact"]');
    await expect(cards).toHaveCount(1);
    await expect(cards.first()).toContainText(parent.title);
    await expect(cards.first()).toContainText("Sub-issues 0/1");
    await expect(page.getByText(child.title, { exact: true })).toHaveCount(0);
    await expect(page.locator("dl").filter({ hasText: "Unfinished" }).locator("div").filter({ hasText: "Unfinished" }).locator("dd")).toHaveText("1");
    await page.screenshot({ path: ".preview/project-parent-tasks.png" });
    await cards.first().click();
    await expect(page.getByTestId("task-workspace")).toBeVisible();
    await chooseDestination(page, child.id);
  });

  test("pauses and resumes through conversation without dispatching the control messages", async ({ page }) => {
    api = await createTestApi();
    const project = await api.createProject("Conversation control");
    const issue = await api.createIssue("Pause the task", { project_id: project.id,
      acceptance_criteria: { v: 1, version: 1, items: [{ id: "", text: "Verify the service", origin: "user", covers: [], checked: null, note: null }], bounds: [], gate: { spec: null, accept: null } },
    });
    const ready = await api.prepareRequirementsReview(issue.id);
    await api.updateIssue(issue.id, { expected_revision: ready.revision, confirm_requirements: true,
      acceptance_criteria: { ...ready.acceptance_criteria, items: ready.acceptance_criteria.items.map((item: Record<string, unknown>) => ({ ...item, checked: { at: "", by: "" } })) } });
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/issues/${issue.id}`);
    await page.getByTestId("comment-composer-shell").click();
    const editor = page.getByRole("textbox", { name: "Comment or instruction for this task", exact: true });
    await editor.fill("暂停部署");
    const pause = page.waitForResponse((response) => response.request().method() === "POST" && response.url().endsWith(`/issues/${issue.id}/execution-control`));
    await page.getByRole("button", { name: "Post comment", exact: true }).click();
    expect((await (await pause).json()).workbench_pause.phase).toBe("paused");
    await expect(page.getByTestId("workspace-action")).toHaveCount(0);
    await expect(page.getByTestId("comment-composer-shell")).toBeVisible();
    await page.getByTestId("comment-composer-shell").click();
    await editor.fill("暂停部署");
    await page.getByRole("button", { name: "Post comment", exact: true }).click();
    await expect(page.getByTestId("comment-composer-shell")).toBeVisible();
    await expect(page.getByText("暂停部署", { exact: true })).toHaveCount(1);
    await page.reload();
    await expect(page.getByTestId("workspace-action")).toHaveCount(0);
    await page.getByTestId("comment-composer-shell").click();
    await editor.fill("继续吧");
    const resume = page.waitForResponse((response) => response.request().method() === "POST" && response.url().endsWith(`/issues/${issue.id}/execution-control`));
    await page.getByRole("button", { name: "Post comment", exact: true }).click();
    expect((await resume).status()).toBe(200);
    await expect(page.getByTestId("workspace-action")).toBeVisible();
  });

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
    await api.prepareRequirementsReview(issue.id);
    await page.reload();
    const bar = page.getByTestId("requirements-summary");
    await expect(bar).toContainText("Confirm requirements");


    for (const size of [{ width: 1440, height: 900 }, { width: 1280, height: 720 }, { width: 1024, height: 768 }]) {
      await page.setViewportSize(size);
      await page.getByRole("button", { name: "Confirm requirements", exact: true }).scrollIntoViewIfNeeded();
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
    await page.getByRole("button", { name: "Confirm requirements", exact: true }).scrollIntoViewIfNeeded();
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
    await page.getByTestId("task-workspace").waitFor();
    await expect(page.locator('[data-message-side="right"]').first()).toBeVisible();
    const ownMessage = page.locator('[data-message-side="right"]').first();
    const ownBounds = await ownMessage.boundingBox();
    const threadBounds = await ownMessage.locator("..").boundingBox();
    expect(ownBounds!.x).toBeGreaterThan(threadBounds!.x + 20);
    expect(Math.abs(ownBounds!.x + ownBounds!.width - threadBounds!.x - threadBounds!.width)).toBeLessThan(3);
    await ownMessage.hover();
    await page.getByRole("button", { name: "Comment actions", exact: true }).first().click();
    await page.getByRole("menuitem", { name: "Reply", exact: true }).click();
    await expect(page.getByRole("button", { name: "Back to task comment" })).toBeVisible();
    await editor.fill("/note A reply draft");
    await expect(page.getByText("Draft saved", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Back to task comment" }).click();
    await expect(page.getByTestId("comment-composer-shell")).toContainText("Add a comment");
    await ownMessage.hover();
    await page.getByRole("button", { name: "Comment actions", exact: true }).first().click();
    await page.getByRole("menuitem", { name: "Reply", exact: true }).click();
    await expect(editor).toHaveText("/note A reply draft");
    await page.getByRole("button", { name: "Post comment", exact: true }).click();
    await expect(page.locator('[data-message-side="right"]').filter({ hasText: "A reply draft" })).toBeVisible();
    await expect(page.getByTestId("reply-composer-shell")).toHaveCount(0);
    expect(errors).toEqual([]);
  });

  test("keeps per-task drafts and posts to the selected child from the parent", async ({ page }) => {
    api = await createTestApi();
    const project = await api.createProject("Unified conversation");
    const parent = await api.createIssue("Coordinate deployment", { project_id: project.id });
    const child = await api.createIssue("Deploy service", { project_id: project.id, parent_issue_id: parent.id });
    const other = await api.createIssue("Verify login", { project_id: project.id, parent_issue_id: parent.id });
    await api.createComment(child.id, "Deployment discussion");
    await api.createComment(other.id, "Verification discussion");
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/issues/${parent.id}`);
    const recipient = page.getByTestId("conversation-recipient");
    await expect(page.getByRole("combobox", { name: "Send to", exact: true })).toHaveCount(0);
    await expect(page.getByText("Deployment discussion", { exact: true })).toBeVisible();
    await expect(page.getByText("Verification discussion", { exact: true })).toBeVisible();
    await page.getByTestId("comment-composer-shell").click();
    const routingEditor = page.getByRole("textbox", { name: "Comment or instruction for this task", exact: true });
    await routingEditor.fill("Check the logs");
    await expect(page.getByText("Which part of the task is this message about?", { exact: true })).toBeVisible();
    await page.getByRole("status").filter({ hasText: "Which part of the task is this message about?" }).getByRole("button", { name: `${child.identifier} · ${child.title}`, exact: true }).click();
    await expect(recipient).toContainText(child.identifier);
    await routingEditor.fill("修改需求");
    await expect(recipient).toContainText(parent.identifier);
    await chooseDestination(page, child.id);
    await page.getByTestId("comment-composer-shell").click();
    const editor = page.getByRole("textbox", { name: "Comment or instruction for this task", exact: true });
    await editor.fill("/note Deployment draft");
    await expect(page.getByText("Draft saved", { exact: true })).toBeVisible();
    await chooseDestination(page, other.id);
    await page.getByTestId("comment-composer-shell").click();
    await editor.fill("/note Verification draft");
    await expect(page.getByText("Draft saved", { exact: true })).toBeVisible();
    await api.updateIssue(other.id, { status: "blocked" });
    await expect(recipient).toContainText(other.identifier);
    await expect(editor).toHaveText("/note Verification draft");
    await chooseDestination(page, child.id);
    await expect(editor).toHaveText("/note Deployment draft");
    const request = page.waitForRequest((req) => req.method() === "POST" && req.url().endsWith(`/issues/${child.id}/comments`));
    await page.getByRole("button", { name: "Post comment", exact: true }).click();
    expect((await request).postDataJSON().content).toContain("Deployment draft");
    await expect(page).toHaveURL(new RegExp(parent.id));
    await expect(page.getByText("Comment posted.", { exact: true })).toBeVisible();
    await chooseDestination(page, other.id);
    await expect(editor).toHaveText("/note Verification draft");
    await page.reload();
    await expect(recipient).toContainText(other.identifier);
    await expect(editor).toHaveText("/note Verification draft");
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
