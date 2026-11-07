/**
 * TestApiClient — lightweight API helper for E2E test data setup/teardown.
 *
 * Uses raw fetch so E2E tests have zero build-time coupling to the web app.
 */

import "./env";
import pg from "pg";

// `||` (not `??`) so an empty `NEXT_PUBLIC_API_URL=` in .env still falls
// back to localhost. dotenv sets unset-vs-empty both as "" — treating them
// the same matches user intent.
const API_BASE = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`;
const DATABASE_URL = process.env.DATABASE_URL ?? "postgres://multica:multica@localhost:5432/multica?sslmode=disable";

interface TestWorkspace {
  id: string;
  name: string;
  slug: string;
}

export type TestIssueStatus =
  | "backlog"
  | "todo"
  | "in_progress"
  | "in_review"
  | "done"
  | "blocked"
  | "cancelled";

export type TestIssuePriority = "urgent" | "high" | "medium" | "low" | "none";

export interface TestTableIssueSeed {
  title: string;
  status?: TestIssueStatus;
  priority?: TestIssuePriority;
  parentIssueId?: string | null;
  position?: number;
}

export interface TestTableIssue {
  id: string;
  title: string;
  status: TestIssueStatus;
  number: number;
}

export class TestApiClient {
  private token: string | null = null;
  private workspaceSlug: string | null = null;
  private workspaceId: string | null = null;
  private email: string | null = null;
  private createdIssueIds: string[] = [];
  private createdProjectIds: string[] = [];
  private createdBusinessTemplateIds: string[] = [];
  private seededIssueIds: string[] = [];
  private createdPropertyIds: string[] = [];
  private releaseFixtureProjectIds: string[] = [];
  private releaseFixturePullRequestIds: string[] = [];

  async login(email: string, name: string) {
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      // Keep each E2E login isolated so previous test runs do not trip the
      // per-email send-code rate limit.
      await client.query("DELETE FROM verification_code WHERE email = $1", [email]);

      // Step 1: Send verification code
      const sendRes = await fetch(`${API_BASE}/auth/send-code`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      if (!sendRes.ok) {
        throw new Error(`send-code failed: ${sendRes.status}`);
      }

      // Step 2: Read code from database
      const result = await client.query(
        "SELECT code FROM verification_code WHERE email = $1 AND used = FALSE AND expires_at > now() ORDER BY created_at DESC LIMIT 1",
        [email],
      );
      if (result.rows.length === 0) {
        throw new Error(`No verification code found for ${email}`);
      }

      const configuredDevCode = process.env.MULTICA_DEV_VERIFICATION_CODE?.trim();
      const code = configuredDevCode || result.rows[0].code;

      // Step 3: Verify code to get JWT
      const verifyRes = await fetch(`${API_BASE}/auth/verify-code`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, code }),
      });
      if (!verifyRes.ok) {
        throw new Error(`verify-code failed: ${verifyRes.status}`);
      }
      const data = await verifyRes.json();

      this.token = data.token;
      this.email = email;

      // Update user name if needed
      if (name && data.user?.name !== name) {
        await this.authedFetch("/api/me", {
          method: "PATCH",
          body: JSON.stringify({ name }),
        });
      }

      await client.query("DELETE FROM verification_code WHERE email = $1", [email]);

      return data;
    } finally {
      await client.end();
    }
  }

  async getWorkspaces(): Promise<TestWorkspace[]> {
    const res = await this.authedFetch("/api/workspaces");
    return res.json();
  }

  setWorkspaceId(id: string) {
    this.workspaceId = id;
  }

  setWorkspaceSlug(slug: string) {
    this.workspaceSlug = slug;
  }

  async ensureWorkspace(name = "E2E Workspace", slug = "e2e-workspace") {
    const workspaces = await this.getWorkspaces();
    const workspace = workspaces.find((item) => item.slug === slug) ?? workspaces[0];
    if (workspace) {
      this.workspaceId = workspace.id;
      this.workspaceSlug = workspace.slug;
      return workspace;
    }

    const res = await this.authedFetch("/api/workspaces", {
      method: "POST",
      body: JSON.stringify({ name, slug }),
    });
    if (res.ok) {
      const created = (await res.json()) as TestWorkspace;
      this.workspaceId = created.id;
      this.workspaceSlug = created.slug;
      return created;
    }

    const refreshed = await this.getWorkspaces();
    const created = refreshed.find((item) => item.slug === slug) ?? refreshed[0];
    if (created) {
      this.workspaceId = created.id;
      this.workspaceSlug = created.slug;
      return created;
    }

    throw new Error(`Failed to ensure workspace ${slug}: ${res.status} ${res.statusText}`);
  }

  async markUserOnboarded() {
    if (!this.email) {
      throw new Error("Cannot mark E2E user onboarded before login");
    }

    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result = await client.query(
        `
          UPDATE "user"
          SET
            onboarded_at = COALESCE(onboarded_at, now()),
            onboarding_questionnaire = COALESCE(onboarding_questionnaire, '{}'::jsonb)
              || '{"source":["friends_colleagues"],"source_other":null,"source_skipped":false}'::jsonb
          WHERE email = $1
        `,
        [this.email],
      );
      if (result.rowCount !== 1) {
        throw new Error(`Failed to mark E2E user onboarded: ${this.email}`);
      }
    } finally {
      await client.end();
    }
  }

  /** Create a project and register it for cleanup. */
  async createProject(title: string, opts?: Record<string, unknown>) {
    const res = await this.authedFetch("/api/projects", {
      method: "POST",
      body: JSON.stringify({ title, ...opts }),
    });
    if (!res.ok) {
      throw new Error(`create project failed: ${res.status} ${await res.text()}`);
    }
    const project = await res.json();
    this.createdProjectIds.push(project.id);
    return project as { id: string; title: string; status: string };
  }

  async createBusinessTemplate(content: Record<string, unknown>) {
    const response = await this.authedFetch("/api/workbench/business-templates", { method: "POST", body: JSON.stringify({ content }) });
    if (!response.ok) throw new Error(`create business template failed: ${response.status} ${await response.text()}`);
    const asset = await response.json();
    this.trackBusinessTemplate(asset.id);
    return asset;
  }

  trackBusinessTemplate(id: string) {
    this.createdBusinessTemplateIds.push(id);
  }

  /** Create a workspace custom property definition; archived on cleanup. */
  async createProperty(name: string, type = "text", config?: { options: { name: string; color: string }[] }) {
    const res = await this.authedFetch("/api/properties", {
      method: "POST",
      body: JSON.stringify({ name, type, config }),
    });
    if (!res.ok) {
      throw new Error(`create property failed: ${res.status} ${await res.text()}`);
    }
    const property = await res.json();
    this.createdPropertyIds.push(property.id);
    return property as { id: string; name: string; config: { options?: { id: string; name: string; color: string }[] } };
  }

  async updateProject(id: string, updates: Record<string, unknown>) {
    const res = await this.authedFetch(`/api/projects/${id}`, {
      method: "PUT",
      body: JSON.stringify(updates),
    });
    if (!res.ok) {
      throw new Error(`update project failed: ${res.status} ${await res.text()}`);
    }
    return res.json();
  }

  async deleteProject(id: string) {
    await this.authedFetch(`/api/projects/${id}`, { method: "DELETE" });
  }

  async createIssue(title: string, opts?: Record<string, unknown>) {
    const res = await this.authedFetch("/api/issues", {
      method: "POST",
      body: JSON.stringify({ title, ...opts }),
    });
    const issue = await res.json();
    this.createdIssueIds.push(issue.id);
    return issue;
  }

  /**
   * Insert a large, deterministic issue fixture in one transaction.
   *
   * Browser E2E coverage for cursor-backed Table views needs 1,000+ rows,
   * which would make setup itself dominate the test if every row went through
   * the HTTP create endpoint. These rows intentionally contain no dependent
   * records; cleanup deletes exactly the returned IDs from the isolated E2E
   * workspace.
   */
  async seedTableIssues(rows: TestTableIssueSeed[]): Promise<TestTableIssue[]> {
    if (rows.length === 0) return [];
    if (!this.workspaceId || !this.email) {
      throw new Error("Cannot seed table issues before login and workspace setup");
    }

    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      await client.query("BEGIN");
      const userResult = await client.query<{ id: string }>(
        `SELECT id FROM "user" WHERE email = $1`,
        [this.email],
      );
      const creatorId = userResult.rows[0]?.id;
      if (!creatorId) {
        throw new Error(`Cannot resolve E2E creator for ${this.email}`);
      }

      const counterResult = await client.query<{ issue_counter: number }>(
        `
          UPDATE workspace
          SET issue_counter = issue_counter + $2
          WHERE id = $1
          RETURNING issue_counter
        `,
        [this.workspaceId, rows.length],
      );
      const finalCounter = Number(counterResult.rows[0]?.issue_counter);
      if (!Number.isFinite(finalCounter)) {
        throw new Error(`Cannot reserve issue numbers for workspace ${this.workspaceId}`);
      }
      const firstNumber = finalCounter - rows.length + 1;

      const inserted = await client.query<TestTableIssue>(
        `
          INSERT INTO issue (
            workspace_id,
            title,
            status,
            priority,
            creator_type,
            creator_id,
            parent_issue_id,
            position,
            number
          )
          SELECT
            $1::uuid,
            fixture.title,
            fixture.status,
            fixture.priority,
            'member',
            $2::uuid,
            fixture.parent_issue_id,
            fixture.position,
            fixture.number
          FROM unnest(
            $3::text[],
            $4::text[],
            $5::text[],
            $6::uuid[],
            $7::double precision[],
            $8::integer[]
          ) WITH ORDINALITY AS fixture(
            title,
            status,
            priority,
            parent_issue_id,
            position,
            number,
            ordinal
          )
          ORDER BY fixture.ordinal
          RETURNING id, title, status, number
        `,
        [
          this.workspaceId,
          creatorId,
          rows.map((row) => row.title),
          rows.map((row) => row.status ?? "backlog"),
          rows.map((row) => row.priority ?? "none"),
          rows.map((row) => row.parentIssueId ?? null),
          rows.map((row, index) => row.position ?? index + 1),
          rows.map((_row, index) => firstNumber + index),
        ],
      );
      await client.query("COMMIT");
      this.seededIssueIds.push(...inserted.rows.map((row) => row.id));
      return inserted.rows;
    } catch (error) {
      await client.query("ROLLBACK");
      throw error;
    } finally {
      await client.end();
    }
  }

  async deleteIssue(id: string) {
    await this.authedFetch(`/api/issues/${id}`, { method: "DELETE" });
  }

  async createComment(issueId: string, content: string) {
    const res = await this.authedFetch(`/api/issues/${issueId}/comments`, {
      method: "POST",
      body: JSON.stringify({ content }),
    });
    if (!res.ok) {
      throw new Error(`create comment failed: ${res.status} ${await res.text()}`);
    }
    return (await res.json()) as { id: string };
  }

  async expireAuditExport(id: string) {
    if (!this.workspaceId) throw new Error("Select a workspace before expiring an audit export");
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result = await client.query(
        "UPDATE workbench_audit_export SET expires_at = now() - interval '1 second' WHERE id = $1 AND workspace_id = $2",
        [id, this.workspaceId],
      );
      if (result.rowCount !== 1) throw new Error("Audit export not found in the selected workspace");
    } finally {
      await client.end();
    }
  }

  async seedCredentialExpiryNotice(projectId: string) {
    if (!this.workspaceId || !this.email) throw new Error("Select a workspace before seeding a credential reminder");
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      await client.query("BEGIN");
      const result = await client.query(
        `INSERT INTO credentials (workspace_id,project_id,label,kind,usage,secret_enc,wrapped_key,last_four,expires_at)
         VALUES ($1,$2,'Staging deployment key','token',ARRAY['deploy'],$3,$4,'1234',date_trunc('milliseconds',now())+interval '6 days') RETURNING id,expires_at`,
        [this.workspaceId, projectId, Buffer.from("test-encrypted-value"), Buffer.from("test-wrapped-key")],
      );
      const credential = result.rows[0];
      await client.query(
        `INSERT INTO inbox_item (workspace_id,recipient_type,recipient_id,type,severity,title,actor_type,details)
         SELECT $1,'member',id,'review_requested','action_required','Staging deployment key','system',$3 FROM "user" WHERE email=$2`,
        [this.workspaceId, this.email, JSON.stringify({ source: "workbench_credential", workbench: "true", credential_id: credential.id, expires_at: credential.expires_at })],
      );
      await client.query("COMMIT");
      return credential.id as string;
    } catch (error) {
      await client.query("ROLLBACK");
      throw error;
    } finally {
      await client.end();
    }
  }

  /** Seed an owned runtime without launching an actual agent tool. */
  async seedRuntime(name: string): Promise<string> {
    if (!this.workspaceId || !this.email) throw new Error("Select a workspace before seeding a runtime");
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result = await client.query(
        `INSERT INTO agent_runtime (workspace_id, name, runtime_mode, provider, status, owner_id, last_seen_at)
         SELECT $1, $2, 'local', 'claude', 'online', id, now() FROM "user" WHERE email = $3 RETURNING id`,
        [this.workspaceId, name, this.email],
      );
      return result.rows[0].id as string;
    } finally {
      await client.end();
    }
  }

  /** Adds an existing user to this client's workspace and returns the member id. */
  async addMemberByEmail(email: string, role: "admin" | "member" = "member"): Promise<string> {
    if (!this.workspaceId) throw new Error("Cannot add a member before a workspace is selected");
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result = await client.query(
        `INSERT INTO member (workspace_id, user_id, role)
         SELECT $1, id, $3 FROM "user" WHERE email = $2
         RETURNING id`,
        [this.workspaceId, email, role],
      );
      if (result.rowCount !== 1) throw new Error(`No user with email ${email}`);
      return result.rows[0].id as string;
    } finally {
      await client.end();
    }
  }

  async removeMember(memberId: string) {
    const res = await this.authedFetch(`/api/workspaces/${this.workspaceId}/members/${memberId}`, {
      method: "DELETE",
    });
    if (!res.ok) {
      throw new Error(`remove member failed: ${res.status} ${await res.text()}`);
    }
  }

  /** Deletes this client's workspace; only for workspaces a test created. */
  async deleteWorkspace() {
    if (!this.workspaceId) return;
    await this.authedFetch(`/api/workspaces/${this.workspaceId}`, { method: "DELETE" });
  }

  /** Server-side issue search, for comparing against what the UI shows. */
  async searchIssues(q: string, opts: { includeClosed?: boolean; limit?: number } = {}) {
    const params = new URLSearchParams({ q, limit: String(opts.limit ?? 20) });
    if (opts.includeClosed) params.set("include_closed", "true");
    const res = await this.authedFetch(`/api/issues/search?${params}`);
    if (!res.ok) {
      throw new Error(`search issues failed: ${res.status} ${await res.text()}`);
    }
    return (await res.json()) as { issues: { id: string; identifier: string; title: string }[] };
  }

  async updateIssue(id: string, updates: Record<string, unknown>) {
    const res = await this.authedFetch(`/api/issues/${id}`, {
      method: "PUT",
      body: JSON.stringify(updates),
    });
    if (!res.ok) {
      throw new Error(`update issue failed: ${res.status} ${await res.text()}`);
    }
    return res.json();
  }

  async saveExecutionPlan(id: string, input: Record<string, unknown>) {
    const res = await this.authedFetch(`/api/issues/${id}/execution-plan`, {
      method: "POST",
      body: JSON.stringify(input),
    });
    if (!res.ok) {
      throw new Error(`save execution plan failed: ${res.status} ${await res.text()}`);
    }
    const issue = await res.json();
    this.createdIssueIds.push(...issue.acceptance_criteria.plan.issues);
    return issue;
  }

  async createProjectRepository(projectId: string, url: string) {
    const response = await this.authedFetch(`/api/projects/${projectId}/resources`, {
      method: "POST", body: JSON.stringify({ resource_type: "github_repo", resource_ref: { url, ref: "main" } }),
    });
    if (!response.ok) throw new Error(`create repository failed: ${response.status} ${await response.text()}`);
    return response.json();
  }

  async recordDeliveryEvidence(parentId: string, expectedRevision: number, evidence: Record<string,unknown>) {
    const response=await this.authedFetch(`/api/issues/${parentId}/delivery-evidence`,{method:"POST",body:JSON.stringify({expected_revision:expectedRevision,evidence})});
    if(!response.ok) throw new Error(`record delivery failed: ${response.status} ${await response.text()}`);
    return response.json();
  }

  async getIssue(id: string) {
    const response=await this.authedFetch(`/api/issues/${id}`);
    if(!response.ok) throw new Error(`get issue failed: ${response.status}`);
    return response.json();
  }

  async acceptDelivery(parentId: string, expectedRevision: number) {
    const review=await this.authedFetch(`/api/issues/${parentId}/delivery-review`);
    if(!review.ok) throw new Error(`read delivery review failed: ${review.status}`);
    const state=await review.json();
    const response=await this.authedFetch(`/api/issues/${parentId}/delivery-review`,{method:"POST",body:JSON.stringify({expected_revision:expectedRevision,action:"accept",checks:state.checks.map((check:{id:string})=>check.id)})});
    if(!response.ok) throw new Error(`accept delivery failed: ${response.status} ${await response.text()}`);
    return response.json();
  }

  async seedReleaseMerge(projectId:string,issueId:string,sourceCommit:string) {
    if(!this.createdProjectIds.includes(projectId) || !this.createdIssueIds.includes(issueId)) throw new Error("Release fixtures require test-owned project and issue");
    this.releaseFixtureProjectIds.push(projectId);
    const client=new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result=await client.query<{id:string}>(`INSERT INTO github_pull_request
        (workspace_id,installation_id,repo_owner,repo_name,pr_number,title,state,html_url,head_sha,base_branch,merge_commit_sha,pr_created_at,pr_updated_at)
        VALUES ($1,12345,'acme',$2,1,'Release fixture','merged',$3,$4,'main',$5,now(),now()) RETURNING id`,
        [this.workspaceId,projectId,`https://github.com/acme/${projectId}/pull/1`,sourceCommit,"b".repeat(40)]);
      const id=result.rows[0]!.id;
      this.releaseFixturePullRequestIds.push(id);
      await client.query("INSERT INTO issue_pull_request (issue_id,pull_request_id) VALUES ($1,$2)",[issueId,id]);
    } finally {await client.end();}
  }

  async recordSecurityScan(input: Record<string, unknown>) {
    const response = await this.authedFetch("/api/security/scans", { method: "POST", body: JSON.stringify(input) });
    if (!response.ok) throw new Error(`record scan failed: ${response.status} ${await response.text()}`);
    return response.json();
  }

  async createDeploymentCredential(projectId: string, kind: string, secret: string) {
    const response = await this.authedFetch("/api/workbench/credentials", {
      method: "POST", body: JSON.stringify({ label: "Deployment fixture", project_id: projectId, kind, usage: ["deploy"], secret }),
    });
    if (!response.ok) throw new Error(`create credential failed: ${response.status} ${await response.text()}`);
    return response.json();
  }

  async createDeployTarget(projectId: string, input: Record<string, unknown>) {
    const response = await this.authedFetch(`/api/projects/${projectId}/deploy-targets`, { method: "POST", body: JSON.stringify(input) });
    if (!response.ok) throw new Error(`create target failed: ${response.status} ${await response.text()}`);
    return response.json();
  }

  async reportRuntimeDeployProbe(runtimeId: string, targetId: string) {
    const heartbeat = await this.authedFetch("/api/daemon/heartbeat", { method: "POST", body: JSON.stringify({ runtime_id: runtimeId }) });
    if (!heartbeat.ok) throw new Error(`heartbeat failed: ${heartbeat.status} ${await heartbeat.text()}`);
    const { pending_deploy_check: check } = await heartbeat.json();
    if (check?.target_id !== targetId) throw new Error("Expected target check was not delivered to its runtime");
    const response = await this.authedFetch(`/api/daemon/runtimes/${runtimeId}/deploy-targets/${targetId}/checks/${check.check_id}`, {
      method: "POST", body: JSON.stringify({ directory_ready: true, tool_ready: true, ms: 12, free_bytes: 1024 * 1024 * 1024 }),
    });
    if (!response.ok) throw new Error(`report target probe failed: ${response.status} ${await response.text()}`);
  }

  async listSecurityChecks(projectId: string) {
    const response = await this.authedFetch(`/api/security/checks?project_id=${encodeURIComponent(projectId)}`);
    if (!response.ok) throw new Error(`list security checks failed: ${response.status} ${await response.text()}`);
    return response.json();
  }

  /** Clean up all issues created during this test. */
  async cleanup() {
    if(this.releaseFixtureProjectIds.length && this.workspaceId) {
      const client=new pg.Client(DATABASE_URL);
      await client.connect();
      try {
        await client.query("BEGIN");
        await client.query("DELETE FROM workbench_release_reservations WHERE workspace_id=$1 AND release_id IN (SELECT id FROM workbench_release_orders WHERE workspace_id=$1 AND project_id=ANY($2::uuid[]))",[this.workspaceId,this.releaseFixtureProjectIds]);
        await client.query("DELETE FROM activity_log WHERE workspace_id=$1 AND details->>'release_id' IN (SELECT id::text FROM workbench_release_orders WHERE workspace_id=$1 AND project_id=ANY($2::uuid[]))",[this.workspaceId,this.releaseFixtureProjectIds]);
        await client.query("DELETE FROM workbench_release_orders WHERE workspace_id=$1 AND project_id=ANY($2::uuid[])",[this.workspaceId,this.releaseFixtureProjectIds]);
        await client.query("DELETE FROM issue_pull_request WHERE pull_request_id=ANY($1::uuid[])",[this.releaseFixturePullRequestIds]);
        await client.query("DELETE FROM github_pull_request WHERE workspace_id=$1 AND id=ANY($2::uuid[])",[this.workspaceId,this.releaseFixturePullRequestIds]);
        await client.query("COMMIT");
      } catch(error) {await client.query("ROLLBACK");throw error;} finally {await client.end();}
      this.releaseFixtureProjectIds=[];
      this.releaseFixturePullRequestIds=[];
    }
    if (this.createdBusinessTemplateIds.length && this.workspaceId) {
      const client = new pg.Client(DATABASE_URL);
      await client.connect();
      try {
        await client.query("DELETE FROM workbench_business_templates WHERE workspace_id=$1 AND id=ANY($2::uuid[])", [this.workspaceId, this.createdBusinessTemplateIds]);
        await client.query("DELETE FROM activity_log WHERE workspace_id=$1 AND details->>'template_id'=ANY($2::text[])", [this.workspaceId, this.createdBusinessTemplateIds]);
      } finally { await client.end(); }
      this.createdBusinessTemplateIds = [];
    }
    if (this.seededIssueIds.length > 0 && this.workspaceId) {
      const client = new pg.Client(DATABASE_URL);
      await client.connect();
      try {
        await client.query(
          `DELETE FROM issue WHERE workspace_id = $1 AND id = ANY($2::uuid[])`,
          [this.workspaceId, this.seededIssueIds],
        );
      } finally {
        await client.end();
      }
      this.seededIssueIds = [];
    }
    for (const id of this.createdIssueIds) {
      try {
        await this.deleteIssue(id);
      } catch {
        /* ignore — may already be deleted */
      }
    }
    this.createdIssueIds = [];
    // Projects last: an issue delete leaves no project reference behind, and
    // dropping the project first would strand the issues in the list.
    for (const id of this.createdProjectIds) {
      try {
        await this.deleteProject(id);
      } catch {
        /* ignore — may already be deleted */
      }
    }
    this.createdProjectIds = [];
    // Definitions cannot be deleted, only archived; archiving takes them out
    // of every picker so later tests see the catalog they expect.
    for (const id of this.createdPropertyIds) {
      try {
        await this.authedFetch(`/api/properties/${id}`, {
          method: "PATCH",
          body: JSON.stringify({ archived: true }),
        });
      } catch {
        /* ignore */
      }
    }
    this.createdPropertyIds = [];
  }

  getToken() {
    return this.token;
  }

  getEmail() {
    if (!this.email) {
      throw new Error("Test API client is not logged in");
    }
    return this.email;
  }

  private async authedFetch(path: string, init?: RequestInit) {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...((init?.headers as Record<string, string>) ?? {}),
    };
    if (this.token) headers["Authorization"] = `Bearer ${this.token}`;
    if (this.workspaceSlug) headers["X-Workspace-Slug"] = this.workspaceSlug;
    else if (this.workspaceId) headers["X-Workspace-ID"] = this.workspaceId;
    return fetch(`${API_BASE}${path}`, { ...init, headers });
  }
}
