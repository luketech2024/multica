import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("OneSpec versioned workflow", () => {
  test.use({ locale: "en-US" });
  let api: TestApiClient;
  test.afterEach(async () => {
    await api?.cleanup();
  });
  test("independent documentation reaches acceptance through separate merge approval", async ({
    page,
  }) => {
    test.setTimeout(120_000);
    api = await createTestApi();
    const project = await api.createProject("OneSpec documentation review", {
      description: "Verify independent documentation stages",
    });
    const repo = await api.createProjectRepository(
      project.id,
      "https://example.test/onespec-docs.git",
    );
    await api.configureOneSpec(project.id, {
      enabled: true,
      overview_repository_id: repo.id,
      expected_revision: 0,
    });
    const issue = await api.createIssue("Document an upstream change", {
      project_id: project.id,
    });
    let state = await api.oneSpec(issue.id);
    await api.oneSpec(issue.id, "kind", {
      expected_revision: state.revision,
      kind: "documentation",
    });
    const slug = await loginAsDefault(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(`/${slug}/issues/${issue.id}`);
    const panel = page.getByRole("region", {
      name: "OneSpec workflow",
      exact: true,
    });
    await expect(panel).toBeVisible();
    await panel
      .getByRole("button", {
        name: "Refresh configuration before starting",
        exact: true,
      })
      .click();
    await expect
      .poll(async () => (await api.oneSpec(issue.id)).flow.generation)
      .toBe(1);
    for (const stage of [
      "requirements",
      "branch",
      "documentation",
      "testing",
      "merge",
      "merging",
    ]) {
      state = await api.oneSpec(issue.id);
      await api.oneSpec(issue.id, "artifact", {
        expected_revision: state.revision,
        artifact: {
          stage,
          summary: `Verified ${stage} in the controlled test executor`,
          refs: [
            {
              repository_id: repo.id,
              path: `onespec/changes/${issue.id}/${stage}.md`,
              commit: "a".repeat(40),
              digest: "b".repeat(64),
            },
          ],
        },
      });
      await page.reload();
      if (["requirements", "merge"].includes(stage)) {
        await panel
          .getByRole("button", { name: "Confirm this version", exact: true })
          .click();
      }
      await panel
        .getByRole("button", { name: "Continue to next stage", exact: true })
        .click();
      await expect
        .poll(async () => (await api.oneSpec(issue.id)).flow.phase)
        .not.toBe(stage);
    }
    await panel
      .getByRole("button", { name: "Confirm this version", exact: true })
      .click();
    await panel
      .getByRole("button", { name: "Continue to next stage", exact: true })
      .click();
    await expect
      .poll(async () => (await api.getIssue(issue.id)).status)
      .toBe("done");
    await expect(
      panel.getByText("Complete · Completed", { exact: true }),
    ).toBeVisible();
    await panel
      .getByRole("combobox", { name: "Stage to reopen", exact: true })
      .click();
    await page
      .getByRole("option", { name: "Verification", exact: true })
      .click();
    await panel
      .getByRole("button", { name: "Reopen selected stage", exact: true })
      .click();
    await expect
      .poll(async () => (await api.oneSpec(issue.id)).flow.phase)
      .toBe("testing");
    await expect(
      panel.getByText("Verification · Awaiting artifacts", { exact: true }),
    ).toBeVisible();
    expect(errors).toEqual([]);
  });
  test("release preparation remains available without a server", async ({
    page,
  }) => {
    api = await createTestApi();
    const project = await api.createProject("OneSpec release without servers", {
      description: "Prepare release before configuring target servers",
    });
    const repo = await api.createProjectRepository(
      project.id,
      "https://example.test/onespec-release.git",
    );
    await api.configureOneSpec(project.id, {
      enabled: true,
      overview_repository_id: repo.id,
      expected_revision: 0,
    });
    const issue = await api.createIssue("Prepare static website release", {
      project_id: project.id,
    });
    let state = await api.oneSpec(issue.id);
    state = await api.oneSpec(issue.id, "kind", {
      expected_revision: state.revision,
      kind: "release",
    });
    await api.oneSpec(issue.id, "artifact", {
      expected_revision: state.revision,
      artifact: {
        stage: "requirements",
        summary: "Publish the selected static website build",
        refs: [
          {
            repository_id: repo.id,
            path: "onespec/release.md",
            commit: "a".repeat(40),
            digest: "b".repeat(64),
          },
        ],
      },
    });
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/issues/${issue.id}`);
    const panel = page.getByRole("region", {
      name: "OneSpec workflow",
      exact: true,
    });
    await panel
      .getByRole("button", { name: "Confirm this version", exact: true })
      .click();
    await panel
      .getByRole("button", { name: "Continue to next stage", exact: true })
      .click();
    await expect(
      panel.getByText("Release preparation · Awaiting artifacts", { exact: true }),
    ).toBeVisible();
    expect((await api.oneSpec(issue.id)).flow.phase).toBe(
      "release_preparation",
    );
    api.trackApplicationReleaseProject(project.id);
    await panel
      .getByRole("button", {
        name: "Create release draft",
        exact: true,
      })
      .click();
    await expect(
      panel.getByRole("link", { name: "Open release center", exact: true }),
    ).toBeVisible();
    const releaseId = (await api.oneSpec(issue.id)).flow.release_id;
    expect(releaseId).toBeTruthy();
    await panel
      .getByRole("link", { name: "Open release center", exact: true })
      .click();
    await expect(page).toHaveURL(new RegExp(`/releases/${releaseId}$`));
    await expect(page.getByRole("textbox", { name: "Release name", exact: true })).toHaveValue("Prepare static website release");
    await expect(page.getByRole("button", { name: "Save draft", exact: true })).toBeEnabled();
  });
  test("feature plan approval gates child creation and archival", async ({
    page,
  }) => {
    test.setTimeout(120_000);
    api = await createTestApi();
    const project = await api.createProject("OneSpec feature flow", {
      description: "Verify design, plan, implementation and archive",
    });
    const repo = await api.createProjectRepository(
      project.id,
      "https://example.test/onespec-feature.git",
    );
    await api.configureOneSpec(project.id, {
      enabled: true,
      overview_repository_id: repo.id,
      expected_revision: 0,
    });
    const issue = await api.createIssue("Implement a small report change", {
      project_id: project.id,
      acceptance_criteria: {
        v: 1,
        items: [{ text: "All report rows are included" }],
        bounds: [],
      },
    });
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/issues/${issue.id}`);
    const panel = page.getByRole("region", {
      name: "OneSpec workflow",
      exact: true,
    });
    const tasks = [
      {
        key: "dev",
        title: "Implement report",
        description: "Controlled fixture execution",
        role: "dev",
        repo_id: repo.id,
        covers: ["A-01"],
        depends_on: [],
        serial_group: "",
        expected: ["verification"],
      },
    ];
    for (const stage of [
      "requirements",
      "design",
      "plan",
      "branch",
      "implementation",
      "testing",
      "archive",
      "archiving",
    ]) {
      let state = await api.oneSpec(issue.id);
      if (stage === "implementation") {
        let parent = await api.getIssue(issue.id);
        const child = await api.getIssue(
          parent.acceptance_criteria.plan.issues[0],
        );
        await api.updateIssue(child.id, {
          status: "in_progress",
          expected_revision: child.revision,
        });
        const started = await api.getIssue(child.id);
        parent = await api.getIssue(issue.id);
        await api.recordDeliveryEvidence(issue.id, parent.revision, {
          issue_id: child.id,
          issue_revision: started.revision,
          commit: "a".repeat(40),
          passed: true,
          summary: "Controlled fixture verified all report rows",
          reports: [
            {
              name: "verification",
              url: "https://example.test/reports/onespec",
            },
          ],
          changes: { files: 1, additions: 1, deletions: 0 },
          preview_url: "https://example.test/preview/onespec",
        });
      }
      state = await api.oneSpec(issue.id);
      await api.oneSpec(issue.id, "artifact", {
        expected_revision: state.revision,
        artifact: {
          stage,
          summary: `Verified ${stage} in the controlled test executor`,
          refs: [
            {
              repository_id: repo.id,
              path: `onespec/changes/${issue.id}/${stage}.md`,
              commit: "a".repeat(40),
              digest: "b".repeat(64),
            },
          ],
          ...(stage === "plan" ? { tasks } : {}),
        },
      });
      await page.reload();
      if (["requirements", "design", "plan", "archive"].includes(stage)) {
        if (stage === "plan")
          await expect(
            panel.getByRole("button", {
              name: "Apply confirmed plan",
              exact: true,
            }),
          ).toHaveCount(0);
        await panel
          .getByRole("button", { name: "Confirm this version", exact: true })
          .click();
      }
      if (stage === "plan") {
        await panel
          .getByRole("button", { name: "Apply confirmed plan", exact: true })
          .click();
        await expect
          .poll(
            async () =>
              (await api.getIssue(issue.id)).acceptance_criteria.plan?.issues
                .length,
          )
          .toBe(1);
      }
      await panel
        .getByRole("button", { name: "Continue to next stage", exact: true })
        .click();
      await expect
        .poll(async () => (await api.oneSpec(issue.id)).flow.phase)
        .not.toBe(stage);
    }
    await panel
      .getByRole("button", { name: "Confirm this version", exact: true })
      .click();
    await panel
      .getByRole("button", { name: "Continue to next stage", exact: true })
      .click();
    await expect
      .poll(async () => (await api.getIssue(issue.id)).status)
      .toBe("done");
  });
});
