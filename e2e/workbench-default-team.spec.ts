import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.use({locale:"en-US", viewport:{width:1440,height:900}});
test("initializes enabled roles, saves bindings and reuses them for a newly enabled role", async ({page}) => {
  let api: TestApiClient | undefined;
  let runtimeId: string | undefined;
  try {
    api = await createTestApi();
    runtimeId = await api.seedRuntime("Default team fixture");
    const project = await api.createProject("Default team persistence");
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    await page.goto(`/${slug}/projects/${project.id}`);
    await page.getByRole("button", {name:"Project settings",exact:true}).first().click();
    const team = page.getByRole("region", {name:"Project team", exact:true});
    await expect(team).toBeVisible();
    await team.getByRole("switch", {name:"Security",exact:true}).click();
    await team.getByRole("combobox", {name:"Default team runtime"}).click();
    await page.getByRole("option", {name:"Default team fixture",exact:true}).click();
    await team.getByRole("button", {name:"Create default team"}).click();
    await expect(team.getByRole("button", {name:"Creating team..."})).not.toBeVisible();
    await expect(team.getByRole("button", {name:"Create default team"})).not.toBeVisible();
    const bindings = await team.getByRole("combobox", {name:"Agent",exact:true}).allTextContents();
    expect(bindings).toHaveLength(5);
    expect(bindings.every(value => !value.includes("Not set"))).toBeTruthy();
    await team.getByRole("button", {name:"Save",exact:true}).click();
    await expect(team.getByRole("button", {name:"Save",exact:true})).not.toBeVisible();
    await page.reload();
    await expect(team).toBeVisible();
    expect(await team.getByRole("combobox", {name:"Agent",exact:true}).allTextContents()).toEqual(bindings);
    await team.getByRole("switch", {name:"Security",exact:true}).click();
    await team.getByRole("button", {name:"Create default team"}).click();
    await expect(team.getByRole("button", {name:"Creating team..."})).not.toBeVisible();
    await expect(team.getByRole("button", {name:"Create default team"})).not.toBeVisible();
    const expanded = await team.getByRole("combobox", {name:"Agent",exact:true}).allTextContents();
    expect(expanded).toHaveLength(6);
    expect(expanded.filter((_,index) => index !== 4)).toEqual(bindings);
    await team.getByRole("button", {name:"Save",exact:true}).click();
    await expect(team.getByRole("button", {name:"Save",exact:true})).not.toBeVisible();
    expect(errors).toEqual([]);
  } finally {
    await api?.cleanup();
    if (api && runtimeId) await api.cleanupRuntimeTeamFixture(runtimeId);
  }
});
