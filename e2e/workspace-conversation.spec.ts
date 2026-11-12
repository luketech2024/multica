import "./env";

import { expect, test } from "@playwright/test";
import pg from "pg";
import { createTestApi } from "./helpers";

// Steering matrices live in the Go handler and composer tests. This browser
// regression covers the real path: a reply in a running agent's thread goes
// into that turn by default, and its receipt follows delivery.
test("workspace steering and pause acknowledgement", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  const api = await createTestApi();
  const db = new pg.Client(process.env.DATABASE_URL);
  await db.connect();
  let runtimeId: string | undefined;
  let agentId: string | undefined;
  try {
    const workspace = (await api.getWorkspaces())[0]!;
    const user = await db.query<{ id: string }>(`SELECT id FROM "user" WHERE email = $1`, [api.getEmail()]);
    const userId = user.rows[0]!.id;
    const runtime = await db.query<{ id: string }>(
      `INSERT INTO agent_runtime (workspace_id, name, runtime_mode, provider, status, device_info, metadata, owner_id, last_seen_at)
       VALUES ($1, 'Steering verification', 'cloud', 'codex', 'online', 'E2E fixture', '{}'::jsonb, $2, now()) RETURNING id`,
      [workspace.id, userId],
    );
    runtimeId = runtime.rows[0]!.id;
    const agent = await db.query<{ id: string }>(
      `INSERT INTO agent (workspace_id, name, description, instructions, runtime_mode, runtime_config, runtime_id, visibility, permission_mode, max_concurrent_tasks, owner_id)
       VALUES ($1, 'Lambda', '', '', 'cloud', '{}'::jsonb, $2, 'private', 'private', 1, $3) RETURNING id`,
      [workspace.id, runtimeId, userId],
    );
    agentId = agent.rows[0]!.id;
    const project = await api.createProject("Steering workspace");
    const parent = await api.createIssue("Coordinate login fix", { project_id: project.id,
      acceptance_criteria: { v: 1, version: 1, items: [{ id: "", text: "Verify login", origin: "user", covers: [], checked: null, note: null }], bounds: [], gate: { spec: null, accept: null } },
    });
    const issue = await api.createIssue("Fix the Safari 17 login page", { project_id: project.id, parent_issue_id: parent.id, status: "in_progress" });
    await db.query(`UPDATE issue SET assignee_type = 'agent', assignee_id = $1 WHERE id = $2`, [agentId, issue.id]);
    const root = await db.query<{ id: string }>(
      `INSERT INTO comment (workspace_id, issue_id, author_type, author_id, content)
       VALUES ($1, $2, 'member', $3, $4) RETURNING id`,
      [workspace.id, issue.id, userId, `[@Lambda](mention://agent/${agentId}) The login page is blank in Safari 17. Find the cause and fix it.`],
    );
    const rootId = root.rows[0]!.id;
    const turn = await db.query<{ id: string }>(
      `INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, trigger_comment_id, delivered_comment_ids, status, dispatched_at, started_at)
       VALUES ($1, $2, $3, $4, ARRAY[$4]::uuid[], 'running', now() - interval '2 minutes', now() - interval '2 minutes') RETURNING id`,
      [agentId, runtimeId, issue.id, rootId],
    );
    const turnId = turn.rows[0]!.id;
    await db.query(
      `INSERT INTO task_supplement_capability (task_id, workspace_id, issue_id, capability) VALUES ($1, $2, $3, 'task-supplement-v1')`,
      [turnId, workspace.id, issue.id],
    );

    await page.addInitScript((token) => {
      localStorage.setItem("multica_token", token!);
      localStorage.setItem("multica:chat:isOpen", "false");
    }, api.getToken());
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(`/${workspace.slug}/issues/${parent.id}`, { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("task-workspace")).toBeVisible({ timeout: 45_000 });

    const shell = page.getByTestId("comment-composer-shell").first();
    // The empty box makes no claim about who a reply goes to; the chip says
    // so once there is text and the recipient is known.
    await shell.click();
    const editor = page.getByRole("textbox", { name: "Comment or instruction for this task", exact: true });
    await editor.fill("This PR only fixes web; leave the desktop login page alone.");
    const chip = page.getByRole("button", { name: "Lambda trigger: Add to current run" });
    await expect(chip).toHaveCount(0);
    await page.getByRole("button", { name: "More sending options" }).click();
    await expect(chip).toBeVisible({ timeout: 15_000 });
    await page.screenshot({ path: testInfo.outputPath("steer-composer.png") });
    await chip.click();
    await expect(page.getByRole("menuitemradio", { name: /Stop and start over/ })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("steer-menu.png") });
    await page.keyboard.press("Escape");
    await page.keyboard.press("Escape");

    const posted = page.waitForResponse((response) => response.request().method() === "POST" && response.url().endsWith(`/api/issues/${issue.id}/comments`));
    await page.getByRole("button", { name: "Send instruction", exact: true }).first().click();
    const response = await posted;
    expect(response.status()).toBe(201);
    const reply = await response.json();
    expect(reply.supplements).toEqual([expect.objectContaining({ task_id: turnId, agent_id: agentId, status: "pending" })]);
    const runs = await db.query(`SELECT id FROM agent_task_queue WHERE issue_id = $1`, [issue.id]);
    expect(runs.rows).toHaveLength(1);
    await expect(page.getByText("Waiting for Lambda to read it")).toBeVisible();
    await expect(page.getByText("Added to Lambda's run")).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("steer-sent.png") });

    await db.query(`UPDATE task_supplement SET status = 'delivered', delivered_at = now() WHERE comment_id = $1`, [reply.id]);
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByText("Read by Lambda")).toBeVisible({ timeout: 30_000 });
    await page.screenshot({ path: testInfo.outputPath("steer-delivered.png") });
    const guidance = page.locator("[data-comment-block]").filter({ hasText: "This PR only fixes web; leave the desktop login page alone." }).last();
    await guidance.hover();
    await guidance.getByRole("button", { name: "Comment actions", exact: true }).click();
    await expect(page.getByRole("menuitem", { name: "Edit", exact: true })).toHaveCount(0);
    await page.getByRole("menuitem", { name: "Send correction", exact: true }).click();
    await expect(page.getByText("Send a new message to correct this guidance.")).toBeVisible();
      await expect(page.getByRole("button", { name: "Pause task", exact: true })).toBeVisible();
      const paused = page.waitForResponse((response) => response.request().method() === "POST" && response.url().endsWith(`/issues/${parent.id}/execution-control`));
      await page.getByRole("button", { name: "Pause task", exact: true }).click();
      expect((await (await paused).json()).workbench_pause.phase).toBe("stopping");
      await expect(page.getByRole("button", { name: "Pause task", exact: true })).toBeDisabled();
      await db.query(`UPDATE agent_task_queue SET result = COALESCE(result, '{}'::jsonb) || '{"workbench_stop_state":"stopped"}'::jsonb WHERE id = $1`, [turnId]);
      await expect(page.getByTestId("workspace-status")).toHaveText("Execution paused", { timeout: 20000 });
      await expect(page.getByRole("button", { name: "Pause task", exact: true })).toHaveCount(0);
  } finally {
    await api.cleanup();
    if (agentId) await db.query(`DELETE FROM agent_task_queue WHERE agent_id = $1`, [agentId]);
    if (agentId) await db.query(`DELETE FROM agent WHERE id = $1`, [agentId]);
    if (runtimeId) await db.query(`DELETE FROM agent_runtime WHERE id = $1`, [runtimeId]);
    await db.end();
  }
});
