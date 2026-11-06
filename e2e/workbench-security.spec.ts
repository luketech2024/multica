import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench security", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("accepts low risks and fixes high risks only with a newer matching scan", async ({ page }) => {
    test.setTimeout(180000);
    api = await createTestApi();
    const roles = Object.fromEntries(["sched", "product", "dev", "test", "sec", "ops"].map((role) => [role, {
      enabled: ["sched", "product", "sec"].includes(role), agent_id: null, runtime_id: null,
    }]));
    const project = await api.createProject("Security evidence review", { description: "Verify persisted risk decisions and scan evidence.", workbench_roles: { v: 1, roles } });
    const repo = await api.createProjectRepository(project.id, "https://example.test/security.git");
    let parent = await api.createIssue("Review export security", { project_id: project.id, acceptance_criteria: { v: 1, items: [{ id: "", text: "Export is safe" }], bounds: [] } });
    parent.acceptance_criteria.items[0].checked = { at: "", by: "" };
    parent = await api.updateIssue(parent.id, { expected_revision: parent.revision, acceptance_criteria: parent.acceptance_criteria, confirm_requirements: true });
    parent = await api.saveExecutionPlan(parent.id, {
      expected_revision: parent.revision, version: 1, idempotency_key: crypto.randomUUID(),
      tasks: [{ key: "scan", title: "Scan export security", description: "", role: "sec", repo_id: repo.id, covers: ["A-01"], depends_on: [], existing_dependencies: [], serial_group: "", expected: ["Security report"] }],
    });
    const child = await api.updateIssue(parent.acceptance_criteria.plan.issues[0], { status: "done" });
    const scan = await api.recordSecurityScan({
      issue_id: child.id, issue_revision: child.revision, repo_id: repo.id, run_id: "baseline", artifact_version: "v1", commit: "a".repeat(40),
      scanned_at: new Date(Date.now() - 1000).toISOString(), status: "completed", report_url: "https://example.test/security/baseline",
      categories: ["secrets", "deps", "authz", "injection"], findings: [
        { fingerprint: "unsafe-sql", title: "Unsafe SQL export", severity: "high", category: "injection", cwe: "CWE-89", location: "export.go:10", recommendation: "Bind query parameters" },
        { fingerprint: "diagnostic-details", title: "Diagnostic details", severity: "low", category: "authz", cwe: "", location: "api.go:20", recommendation: "Remove internal details" },
      ],
    });
    const risks = await api.listSecurityChecks(project.id);
    const high = risks.checks.find((check: { severity: string }) => check.severity === "high");
    const low = risks.checks.find((check: { severity: string }) => check.severity === "low");
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/security?project=${project.id}&check=${low.id}`);
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("Diagnostic details", { exact: false }).first()).toBeVisible({ timeout: 30000 });
    await dialog.getByRole("combobox", { name: "Status", exact: true }).click();
    await page.getByRole("option", { name: "Accept risk", exact: true }).click();
    await dialog.getByRole("textbox", { name: "Reason", exact: true }).fill("Restricted test account reviewed");
    await dialog.getByRole("button", { name: "Save", exact: true }).click();
    await expect(dialog.getByText("Restricted test account reviewed", { exact: true })).toBeVisible();
    await page.reload();
    await expect(dialog.getByText("Restricted test account reviewed", { exact: true })).toBeVisible();
    await page.goto(`/${slug}/security?project=${project.id}&check=${high.id}`);
    await dialog.getByRole("textbox", { name: "Artifact version", exact: true }).fill("v2");
    await dialog.getByRole("button", { name: "Save", exact: true }).click();
    await expect(dialog.getByText(/High · Injection · Awaiting recheck/)).toBeVisible();
    await dialog.getByRole("combobox", { name: "Status", exact: true }).click();
    await page.getByRole("option", { name: "Verify scan", exact: true }).click();
    await dialog.getByRole("textbox", { name: "Scan record ID", exact: true }).fill(scan.id);
    await dialog.getByRole("button", { name: "Save", exact: true }).click();
    await expect(dialog.getByRole("alert")).toContainText("target artifact");
    await page.goto(`/${slug}/issues/${parent.id}/pipeline?stage=sec&task=${child.id}`);
    await page.locator("summary").filter({ hasText: "Record scan report" }).click();
    await page.getByRole("textbox", { name: "Scan run ID", exact: true }).fill("fixed-v2");
    await page.getByRole("textbox", { name: "Artifact version", exact: true }).fill("v2");
    await page.getByRole("textbox", { name: "Source commit", exact: true }).fill("b".repeat(40));
    const localTime = new Date(Date.now() - new Date().getTimezoneOffset() * 60_000).toISOString().slice(0, 19);
    await page.getByLabel("Scan time", { exact: true }).fill(localTime);
    await page.getByRole("textbox", { name: "Report URL", exact: true }).fill("https://example.test/security/fixed-v2");
    for (const category of ["Secrets and credentials", "Dependencies", "Authorization", "Injection"]) await page.getByRole("checkbox", { name: category, exact: true }).check();
    const recorded = page.waitForResponse((response) => response.request().method() === "POST" && response.url().endsWith("/api/security/scans"));
    await page.getByRole("button", { name: "Record scan report", exact: true }).click();
    const response = await recorded;
    expect(response.status()).toBe(200);
    const fixed = await response.json();
    await expect(page.getByRole("status").filter({ hasText: "Scan report recorded" })).toBeVisible();
    await page.goto(`/${slug}/security?project=${project.id}&check=${high.id}`);
    await dialog.getByRole("combobox", { name: "Status", exact: true }).click();
    await page.getByRole("option", { name: "Verify scan", exact: true }).click();
    await dialog.getByRole("textbox", { name: "Scan record ID", exact: true }).fill(fixed.id);
    await dialog.getByRole("button", { name: "Save", exact: true }).click();
    await expect(dialog.getByText(/High · Injection · Fixed/)).toBeVisible();
    await expect(dialog.getByRole("link", { name: "fixed-v2" })).toHaveAttribute("href", "https://example.test/security/fixed-v2");
    await page.reload();
    await expect(dialog.getByText(/High · Injection · Fixed/)).toBeVisible();
    await page.screenshot({ path: ".preview/security-fixed.png", fullPage: true });
    expect(errors).toEqual([]);
  });
});
