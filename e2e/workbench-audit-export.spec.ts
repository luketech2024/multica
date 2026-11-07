import { readFile } from "node:fs/promises";
import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench audit export", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("downloads the audit snapshot through its inbox notification", async ({ page }) => {
    test.setTimeout(120000);
    api = await createTestApi();
    const project = await api.createProject("Audit inbox project", {
      description: "Verify audit snapshots downloaded through inbox notifications.",
      workbench_roles: { v: 1, roles: Object.fromEntries(["sched", "product", "dev", "test", "sec", "ops"].map((role) => [role, {
        enabled: ["sched", "product"].includes(role), agent_id: null, runtime_id: null,
      }])) },
    });
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/security?tab=audit`);
    await expect(page.getByRole("tab", { name: "Audit log", exact: true })).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("cell", { name: "Project roles created", exact: true }).first()).toBeVisible({ timeout: 30000 });
    const responsePromise = page.waitForResponse((response) => response.url().includes("/api/workbench/audit-exports?") && response.request().method() === "POST");
    await page.getByRole("button", { name: "Export audit", exact: true }).click();
    const response = await responsePromise;
    expect(response.status()).toBe(201);
    const exported = await response.json();
    await page.getByRole("link", { name: "The download is in your inbox" }).click();
    await page.getByRole("button", { name: new RegExp(exported.filename.replaceAll(".", "\\.")) }).first().click();
    await expect(page).toHaveURL(new RegExp(`export=${exported.id}`));
    const readDownload = async () => {
      const pending = page.waitForEvent("download");
      await page.getByRole("button", { name: "Download CSV", exact: true }).click();
      const download = await pending;
      return readFile((await download.path())!, "utf8");
    };
    const csv = await readDownload();
    expect(csv).toContain(project.id);
    expect(csv).toContain("project_roles_created");
    await page.reload();
    expect(await readDownload()).toBe(csv);
    await api.expireAuditExport(exported.id);
    const expiredResponse = page.waitForResponse((response) => response.url().endsWith(`/api/workbench/audit-exports/${exported.id}`));
    await page.getByRole("button", { name: "Download CSV", exact: true }).click();
    expect((await expiredResponse).status()).toBe(410);
    await expect(page.getByRole("alert").filter({ hasText: "audit export expired" })).toBeVisible();
    await page.screenshot({ path: ".preview/audit-download.png", fullPage: true });
    expect(errors).toEqual([]);
  });

  test("downloads persisted configuration changes from settings", async ({ page }) => {
    test.setTimeout(90000);
    api = await createTestApi();
    const roles = Object.fromEntries(["sched", "product", "dev", "test", "sec", "ops"].map((role) => [role, {
      enabled: ["sched", "product"].includes(role), agent_id: null, runtime_id: null,
    }]));
    const project = await api.createProject("Audit CSV project", {
      description: "Verify persisted configuration changes in the downloaded audit.",
      workbench_roles: { v: 1, roles },
    });
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/settings?tab=code`);
    const exportButton = page.getByRole("button", { name: "Export configuration audit (90 days)" });
    await expect(exportButton).toBeVisible({ timeout: 30000 });
    const downloadPromise = page.waitForEvent("download");
    await exportButton.click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toMatch(/^workbench-audit-\d{4}-\d{2}-\d{2}\.csv$/);
    const path = await download.path();
    expect(path).not.toBeNull();
    const csv = await readFile(path!, "utf8");
    expect(csv).toContain("id,created_at,actor_type,actor_id,actor_name,issue_id,action,details");
    expect(csv).toContain("project_roles_created");
    expect(csv).toContain(project.id);
    expect(errors).toEqual([]);
  });
});
