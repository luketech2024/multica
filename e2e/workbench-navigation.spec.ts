import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workbench navigation", () => {
  test.use({ locale: "en-US", viewport: { width: 1440, height: 900 } });
  let api: TestApiClient;
  test.afterEach(async () => {
    await api?.cleanup();
  });
  test("moves pins, restores project board context and collapses at 1279px", async ({
    page,
  }) => {
    test.setTimeout(150000);
    api = await createTestApi();
    const project = await api.createProject("Navigation project");
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/projects/${project.id}`);
    const nav = page.locator('nav[data-slot="app-rail"]');
    const entries = nav.locator("[data-workbench-navigation] a");
    await expect(entries).toHaveCount(14);
    await expect(
      nav.getByRole("link", { name: "Projects", exact: true }),
    ).toHaveAttribute("aria-current", "page");
    await nav.getByRole("button", { name: "Unpin Inbox", exact: true }).click();
    await nav
      .getByRole("button", { name: "Pin Projects", exact: true })
      .click();
    await expect(
      nav
        .getByRole("region", { name: "Pinned", exact: true })
        .getByRole("link", { name: "Projects", exact: true }),
    ).toBeVisible();
    const savedPins = () =>
      page.evaluate(
        (workspaceSlug) =>
          JSON.parse(
            localStorage.getItem(
              `multica_workbench_navigation:${workspaceSlug}`,
            ) ?? "{}",
          ).state?.pinned,
        slug,
      );
    expect(await savedPins()).toEqual(["my_issues", "issues", "projects"]);
    await page.reload();
    expect(await savedPins()).toEqual(["my_issues", "issues", "projects"]);
    await expect(
      nav
        .getByRole("region", { name: "Pinned", exact: true })
        .getByRole("link", { name: "Projects", exact: true }),
    ).toBeVisible();
    await expect(entries).toHaveCount(14);
    await nav.getByRole("link", { name: "Task board", exact: true }).click();
    await expect(page).toHaveURL(
      new RegExp(`/issues\\?layout=board&project=${project.id}(&|$)`),
    );
    await expect(
      page.getByRole("combobox", { name: "Projects", exact: true }),
    ).toContainText(project.title);
    await expect(
      nav.getByRole("link", { name: "Task board", exact: true }),
    ).toHaveAttribute("aria-current", "page");
    await nav.getByRole("link", { name: "Issues", exact: true }).click();
    await expect(page).toHaveURL(/\/issues\?layout=table$/);
    await expect(
      nav.getByRole("link", { name: "Issues", exact: true }),
    ).toHaveAttribute("aria-current", "page");
    await page.goto(`/${slug}/projects/${project.id}?tab=autopilots`);
    await expect(
      page.getByRole("tab", { name: "Autopilot", exact: true }),
    ).toHaveAttribute("aria-selected", "true");
    await expect(
      page.getByRole("button", { name: "New autopilot", exact: true }).first(),
    ).toBeVisible();
    await page.screenshot({
      path: ".preview/workbench-navigation-wide.png",
      fullPage: true,
    });
    await page.setViewportSize({ width: 1024, height: 720 });
    await expect(nav).toHaveAttribute("data-expanded", "false");
    expect(
      await nav.evaluate((node) => node.getBoundingClientRect().width),
    ).toBe(64);
    await expect(entries).toHaveCount(14);
    await nav.getByRole("link", { name: "Agent chat", exact: true }).hover();
    await expect(
      page
        .locator('[data-slot="tooltip-content"]')
        .filter({ hasText: /^Agent chat$/ }),
    ).toBeVisible();
    await page.screenshot({
      path: ".preview/workbench-navigation-compact.png",
      fullPage: true,
    });
    expect(errors).toEqual([]);
  });
});
