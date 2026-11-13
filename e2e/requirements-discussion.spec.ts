import "./env";
import { expect, test } from "@playwright/test";
import pg from "pg";
import { createTestApi } from "./helpers";

test("discuss before confirming, retain checks after answers, and pause only the reply", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  page.setDefaultTimeout(15_000);
  const api = await createTestApi();
  const workspace = (await api.getWorkspaces())[0]!;
  const db = new pg.Client({ connectionString: process.env.DATABASE_URL });
  await db.connect();
  let agentId: string | undefined;
  let runtimeId: string | undefined;
  try {
    const user = await api.authedFetch("/api/me");
    const userId = (await user.json()).id;
    runtimeId = (await db.query(`INSERT INTO agent_runtime (workspace_id,name,runtime_mode,provider,status,device_info,metadata,owner_id,last_seen_at)
      VALUES ($1,'Discussion fixture','cloud','codex','online','E2E','{}',$2,now()) RETURNING id`, [workspace.id, userId])).rows[0].id;
    agentId = (await db.query(`INSERT INTO agent (workspace_id,name,description,instructions,runtime_mode,runtime_config,runtime_id,visibility,permission_mode,max_concurrent_tasks,owner_id)
      VALUES ($1,'Discussion agent','','','cloud','{}',$2,'private','private',1,$3) RETURNING id`, [workspace.id, runtimeId, userId])).rows[0].id;
    const project = await api.createProject("Requirements discussion");
    const issue = await api.createIssue("Deploy an existing website", { project_id: project.id, assignee_type: "agent", assignee_id: agentId });
    expect((await db.query(`SELECT id FROM agent_task_queue WHERE issue_id=$1`, [issue.id])).rows).toHaveLength(0);
    await page.addInitScript((token) => { localStorage.setItem("multica_token", token!); localStorage.setItem("multica:chat:isOpen", "false"); }, api.getToken());
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${workspace.slug}/issues/${issue.id}`, { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("workspace-status")).toHaveText("Discussing requirements", { timeout: 30_000 });
    await expect(page.getByText("Tell us what you want to accomplish, even if it is just an initial idea.")).toBeVisible();
    await expect(page.getByTestId("requirements-summary")).toHaveCount(0);
    const editor = page.getByRole("textbox", { name: "Comment or instruction for this task", exact: true });
    await expect(editor).toBeFocused();
    await page.screenshot({ path: testInfo.outputPath("discussion-start.png") });
    await editor.fill("Deploy the existing website in a test environment and verify the homepage.");
    await page.getByRole("button", { name: "Send instruction", exact: true }).click();
    await expect.poll(async () => (await db.query(`SELECT count(*)::int AS n FROM agent_task_queue WHERE issue_id=$1`, [issue.id])).rows[0].n).toBe(1);
    await page.getByRole("button", { name: "Pause task", exact: true }).click();
    await expect(page.getByTestId("workspace-status")).toHaveText("Reply paused");
    expect((await api.getIssue(issue.id)).workbench_pause).toBeFalsy();
    await page.getByTestId("comment-composer-shell").click();
    await editor.fill("The homepage is enough for this test.");
    await page.getByRole("button", { name: "Send instruction", exact: true }).click();
    await expect.poll(async () => (await db.query(`SELECT count(*)::int AS n FROM agent_task_queue WHERE issue_id=$1`, [issue.id])).rows[0].n).toBe(2);
    await db.query(`UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE issue_id=$1`, [issue.id]);
    const items = [{ id: "A-01", text: "Open the homepage in the test environment", origin: "product", covers: [], checked: null, note: null }];
    await db.query(`UPDATE issue SET acceptance_criteria=acceptance_criteria || jsonb_build_object('phase','draft','draft_revision',1,'items',$2::jsonb),revision=revision+1 WHERE id=$1`, [issue.id, JSON.stringify(items)]);
    await page.reload();
    const card = page.getByTestId("requirements-summary");
    await expect(card).toContainText("Requirements draft");
    await expect(card.getByRole("checkbox")).toHaveCount(0);
    await expect(page.getByTestId("comment-composer-shell")).toHaveCount(1);
    await expect(page.locator("textarea")).toHaveCount(0);
    await db.query(`UPDATE issue SET acceptance_criteria=acceptance_criteria || jsonb_build_object('phase','ready','ready_revision',1,'covered_discussion_revision',acceptance_criteria->'discussion_revision'),revision=revision+1 WHERE id=$1`, [issue.id]);
    await page.reload();
    await expect(card).toContainText("Requirements ready for review");
    await expect(page.getByTestId("comment-composer-shell")).toHaveCount(1);
    await expect(page.locator("textarea")).toHaveCount(0);
    await card.getByRole("checkbox").click();
    await expect(card.getByRole("checkbox")).toBeChecked();
    await expect(card.getByRole("button", { name: "Confirm requirements", exact: true })).toBeEnabled();
    await page.screenshot({ path: testInfo.outputPath("discussion-ready.png") });
    await api.createComment(issue.id, "Will this change the production website?");
    await expect(card).toContainText("Processing your additions");
    await expect(card.getByRole("button", { name: "Confirm requirements", exact: true })).toHaveCount(0);
    await db.query(`UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE issue_id=$1`, [issue.id]);
    await db.query(`UPDATE issue SET acceptance_criteria=jsonb_set(acceptance_criteria,'{covered_discussion_revision}',acceptance_criteria->'discussion_revision'),revision=revision+1 WHERE id=$1`, [issue.id]);
    await page.reload();
    await expect(card.getByRole("checkbox")).toBeChecked();
    await card.getByRole("button", { name: "Confirm requirements", exact: true }).click();
    await expect.poll(async () => (await api.getIssue(issue.id)).acceptance_criteria?.gate.spec).toBeTruthy();
    const confirmation = page.getByTestId("workspace-confirmed-requirements");
    await expect(confirmation).toBeVisible();
    const execution = (await db.query(`SELECT id FROM agent_task_queue WHERE issue_id=$1 ORDER BY created_at DESC LIMIT 1`, [issue.id])).rows[0].id;
    const executionRow = page.locator(`[data-conversation-row="${execution}"]`);
    await expect(executionRow).toBeVisible();
    const expectConfirmationFirst = async () => {
      await expect.poll(() => confirmation.evaluate((element, id) => {
        const run = document.querySelector(`[data-conversation-row="${id}"]`);
        return !!run && !!(element.compareDocumentPosition(run) & Node.DOCUMENT_POSITION_FOLLOWING);
      }, execution)).toBe(true);
    };
    await expectConfirmationFirst();
    await page.reload();
    await expect(executionRow).toBeVisible();
    await expectConfirmationFirst();
    await db.query(`UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, [execution]);
    await page.reload();
    await expect(executionRow).toBeVisible();
    await expectConfirmationFirst();
    await page.screenshot({ path: testInfo.outputPath("confirmation-before-execution.png") });
  } finally {
    await api.cleanup();
    if (agentId) await db.query(`DELETE FROM agent_task_queue WHERE agent_id=$1`, [agentId]);
    if (agentId) await db.query(`DELETE FROM agent WHERE id=$1`, [agentId]);
    if (runtimeId) await db.query(`DELETE FROM agent_runtime WHERE id=$1`, [runtimeId]);
    await db.end();
  }
});
