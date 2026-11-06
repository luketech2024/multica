import { readFile } from "node:fs/promises";
import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench audit export", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

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
