import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Deployment target checks", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => { await api?.cleanup(); });

  test("queues a configured target and displays the runtime result after refresh", async ({ page }) => {
    test.setTimeout(120000);
    api = await createTestApi();
    const project = await api.createProject("Deployment self-check");
    const repo = await api.createProjectRepository(project.id, "https://example.test/deployment-check.git");
    const runtimeId = await api.seedRuntime("Deployment probe fixture");
    const target = await api.createDeployTarget(project.id, {
      label: "Staging service", repo_id: repo.id, env: "staging", kind: "service-process", channel: "runtime", runtime_id: runtimeId,
      location: { host: "configured-host", dir: "/srv/service", start_command: "service-server", health_url: "https://example.test/health" },
    });
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/projects/${project.id}`);
    await page.getByRole("tab", { name: /Environment and deployment/ }).click();
    const section = page.getByRole("region", { name: "Deployment targets", exact: true });
    await expect(section).toBeVisible({ timeout: 30000 });
    const queued = page.waitForResponse((response) => response.url().endsWith(`/deploy-targets/${target.id}/check`));
    await section.getByRole("button", { name: "Check connection", exact: true }).click();
    expect((await queued).status()).toBe(202);
    await expect(section.getByText("Waiting for runtime", { exact: true })).toBeVisible();
    await api.reportRuntimeDeployProbe(runtimeId, target.id);
    await expect(section.getByText("Passed · valid for 24 hours", { exact: true })).toBeVisible({ timeout: 15000 });
    await page.reload();
    await expect(page.getByRole("tab", { name: /Environment and deployment/ })).toHaveAttribute("aria-selected", "true");
    await expect(section.getByText("Passed · valid for 24 hours", { exact: true })).toBeVisible();
    await section.getByRole("combobox", { name: "Environment", exact: true }).click();
    await page.getByRole("option", { name: "Production", exact: true }).click();
    await expect(section.getByText("Staging service", { exact: true })).not.toBeVisible();
    await expect(section.getByRole("button", { name: "Bind target", exact: true })).toBeVisible();
    await section.getByRole("combobox", { name: "Environment", exact: true }).click();
    await page.getByRole("option", { name: "Staging", exact: true }).click();
    await section.getByRole("button", { name: "Edit", exact: true }).click();
    const automatic = page.waitForResponse((response) => response.url().endsWith(`/deploy-targets/${target.id}/check`));
    await section.getByRole("button", { name: "Save", exact: true }).click();
    expect((await automatic).status()).toBe(202);
    await api.reportRuntimeDeployProbe(runtimeId, target.id);
    await expect(section.getByText("Passed · valid for 24 hours", { exact: true })).toBeVisible({ timeout: 15000 });
    await expect(section.getByText("Available disk space · 1,024 MiB", { exact: true })).toBeVisible();
    await page.screenshot({ path: ".preview/deployment-pane.png", fullPage: true });
    expect(errors).toEqual([]);
  });
});
