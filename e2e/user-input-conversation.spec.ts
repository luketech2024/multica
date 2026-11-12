import "./env";
import { expect, test } from "@playwright/test";
import pg from "pg";
import { createTestApi } from "./helpers";

test("answers agent questions in the shared composer across reloads", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  const api = await createTestApi();
  const db = new pg.Client(process.env.DATABASE_URL);
  await db.connect();
  let runtimeId: string | undefined;
  let agentId: string | undefined;
  try {
    const workspace = (await api.getWorkspaces())[0]!;
    const userId = (await db.query(`SELECT id FROM "user" WHERE email=$1`, [api.getEmail()])).rows[0].id;
    runtimeId = (await db.query(`INSERT INTO agent_runtime(workspace_id,name,runtime_mode,provider,status,device_info,metadata,owner_id,last_seen_at) VALUES ($1,'Question test','cloud','codex','online','E2E','{}',$2,now()) RETURNING id`, [workspace.id,userId])).rows[0].id;
    agentId = (await db.query(`INSERT INTO agent(workspace_id,name,description,instructions,runtime_mode,runtime_config,runtime_id,visibility,permission_mode,max_concurrent_tasks,owner_id) VALUES ($1,'Question agent','','','cloud','{}',$2,'private','private',1,$3) RETURNING id`, [workspace.id,runtimeId,userId])).rows[0].id;
    const project = await api.createProject("Conversation questions");
    const parent = await api.createIssue("Configure a static website", {project_id:project.id,
      acceptance_criteria: { v:1,version:1,items:[{id:"",text:"Verify website",origin:"user",covers:[],checked:null,note:null}],bounds:[],gate:{spec:null,accept:null} },
    });
    const child = await api.createIssue("Confirm website settings", {project_id:project.id,parent_issue_id:parent.id,status:"in_progress"});
    await db.query(`UPDATE issue SET assignee_type='agent', assignee_id=$1 WHERE id=$2`,[agentId,child.id]);
    const taskId = (await db.query(`INSERT INTO agent_task_queue(agent_id,runtime_id,issue_id,status,dispatched_at,started_at) VALUES($1,$2,$3,'running',now(),now()) RETURNING id`,[agentId,runtimeId,child.id])).rows[0].id;
    const apiBase = process.env.NEXT_PUBLIC_API_URL!;
    const headers = {Authorization:`Bearer ${api.getToken()}`,"Content-Type":"application/json","X-Workspace-ID":workspace.id};
    const question = {
      key:"thread/turn/first",threadId:"thread",turnId:"turn",
      questions:[
        {id:"port",header:"Port",question:"Which port should the website use?",isOther:false,options:[{label:"8080",description:"Application port"},{label:"8081",description:"Alternative port"}]},
        {id:"domain",header:"Domain",question:"Which domain should point to the website?",isOther:true,options:[]},
      ],
    };
    const created = await fetch(`${apiBase}/api/daemon/tasks/${taskId}/input-requests`,{method:"POST",headers,body:JSON.stringify(question)});
    expect(created.status).toBe(200);
    const request = await created.json();
    await page.addInitScript((token) => localStorage.setItem("multica_token",token!),api.getToken());
    await page.setViewportSize({width:1440,height:1000});
    await page.goto(`/${workspace.slug}/issues/${parent.id}`,{waitUntil:"domcontentloaded"});
    const composer = page.getByTestId("user-input-composer");
    await expect(composer).toBeVisible({timeout:45_000});
    await expect(page.getByTestId("workspace-status")).toHaveText("Waiting for your answer");
    await composer.getByRole("button",{name:"8080",exact:true}).click();
    await expect(composer.getByRole("textbox",{name:"Answer",exact:true})).toHaveText("8080");
    await composer.getByRole("button",{name:"Answer",exact:true}).click();
    await expect(composer.getByText(/Answer · Domain/)).toBeVisible();
    await page.reload({waitUntil:"domcontentloaded"});
    await expect(composer.getByText(/Answer · Domain/)).toBeVisible({timeout:30_000});
    await composer.getByTestId("comment-composer-shell").click();
    await composer.getByRole("textbox",{name:"Answer",exact:true}).fill("example.test");
    const answered = page.waitForResponse((response) => response.url().endsWith(`/input-requests/${request.id}/answer`) && response.request().method()==="POST");
    await composer.getByRole("button",{name:"Answer",exact:true}).click();
    expect((await answered).status()).toBe(200);
    await expect(composer).toHaveCount(0);
    await expect(page.getByText("example.test",{exact:true})).toBeVisible();
    const received = await fetch(`${apiBase}/api/daemon/tasks/${taskId}/input-requests/${request.id}`,{headers});
    expect((await received.json()).response.answers).toEqual({port:{answers:["8080"]},domain:{answers:["example.test"]}});
    expect((await db.query(`SELECT count(*)::int AS n FROM agent_task_queue WHERE issue_id=$1`,[child.id])).rows[0].n).toBe(1);
    await page.screenshot({path:testInfo.outputPath("answered-in-conversation.png")});

    const next = await fetch(`${apiBase}/api/daemon/tasks/${taskId}/input-requests`,{method:"POST",headers,body:JSON.stringify({...question,key:"thread/turn/second",questions:[question.questions[0]]})});
    expect(next.status).toBe(200);
    const nextRequest = await next.json();
    await expect(composer).toBeVisible({timeout:10_000});
    await composer.getByRole("button",{name:"Pause task",exact:true}).click();
    await expect(composer).toHaveCount(0,{timeout:15_000});
    const late = await fetch(`${apiBase}/api/issues/${child.id}/input-requests/${nextRequest.id}/answer`,{method:"POST",headers,body:JSON.stringify({answers:{port:{answers:["8080"]}}})});
    expect(late.status).toBe(409);
  } finally {
    await api.cleanup();
    if (agentId) await db.query(`DELETE FROM agent_task_queue WHERE agent_id=$1`,[agentId]);
    if (agentId) await db.query(`DELETE FROM agent WHERE id=$1`,[agentId]);
    if (runtimeId) await db.query(`DELETE FROM agent_runtime WHERE id=$1`,[runtimeId]);
    await db.end();
  }
});
