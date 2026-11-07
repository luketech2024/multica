import { test, expect } from "@playwright/test";
import { loginAsDefault } from "./helpers";

test("authenticated home opens projects and issue links retain their destination", async ({ page, context }) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const slug = await loginAsDefault(page);
  await context.addCookies([
    { name: "multica_logged_in", value: "1", url: page.url() },
    { name: "last_workspace_slug", value: slug, url: page.url() },
  ]);
  await page.goto("/");
  await expect(page).toHaveURL(new RegExp(`/${slug}/projects$`));
  await expect(page.getByRole("main")).toBeVisible();
  await page.reload();
  await expect(page).toHaveURL(new RegExp(`/${slug}/projects$`));
  await page.goto("/issues?wb_mine=1");
  await expect(page).toHaveURL(new RegExp(`/${slug}/issues\\?wb_mine=1$`));
  expect(errors).toEqual([]);
});
