import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";

test.use({ locale: "en-US" });

test("keeps delivery in conversation order and refreshes acceptance after follow-up changes", async ({ page }) => {
  test.setTimeout(120_000);
  const api = await createTestApi();
  try {
    const project = await api.createProject("Delivery follow-up regression");
    let parent = await api.createIssue("Deploy and review", { project_id: project.id,
      acceptance_criteria: { v: 1, items: [{ text: "Service is reachable" }], bounds: [] },
    });
    parent = await api.prepareRequirementsReview(parent.id);
    parent = await api.updateIssue(parent.id, { expected_revision: parent.revision, confirm_requirements: true,
      acceptance_criteria: { ...parent.acceptance_criteria, items: parent.acceptance_criteria.items.map((item: Record<string, unknown>) => ({ ...item, checked: { at: "", by: "" } })) },
    });
    parent = await api.saveExecutionPlan(parent.id, { expected_revision: parent.revision, version: parent.acceptance_criteria.version, idempotency_key: "delivery-followup",
      tasks: [{ key: "deploy", title: "Deploy service", description: "", role: "ops", repo_id: "", covers: ["A-01"], depends_on: [], serial_group: "", expected: ["Deployment report"] }],
    });
    const childId = parent.acceptance_criteria.plan.issues[0];
    parent = await api.updateIssue(parent.id, { expected_revision: parent.revision, status: "in_progress" });
    let child = await api.updateIssue(childId, { status: "done" });
    const evidence = { issue_id: childId, issue_revision: child.revision, summary: "Deployment verified", passed: true,
      reports: [{ name: "Deployment report", url: "https://example.test/deployment" }] };
    parent = await api.recordDeliveryEvidence(parent.id, parent.revision, evidence);
    parent = await api.updateIssue(parent.id, { expected_revision: parent.revision, status: "in_review", assignee_type: "member", assignee_id: parent.creator_id });
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/issues/${parent.id}`);
    const acceptance = page.getByRole("region", { name: "Delivery acceptance", exact: true });
    const accept = acceptance.getByRole("button", { name: "Accept delivery", exact: true });
    await expect(accept).toBeEnabled({ timeout: 30_000 });
    const followup = await api.createComment(childId, "Please explain the delivered version.");
    const reply = page.locator(`[data-conversation-row="${followup.id}"]`);
    await expect(reply).toBeVisible();
    const assertOrder = async () => {
      await expect(page.getByTestId("workspace-delivery")).toBeVisible();
      expect(await page.getByTestId("workspace-delivery").evaluate((element, id) => {
        const following = document.querySelector(`[data-conversation-row="${id}"]`);
        return !!following && !!(element.compareDocumentPosition(following) & Node.DOCUMENT_POSITION_FOLLOWING);
      }, followup.id)).toBe(true);
    };
    await assertOrder();
    await page.reload();
    await assertOrder();
    await expect(accept).toBeEnabled();
    child = await api.updateIssue(childId, { title: "Verify deployment source" });
    await expect(accept).toBeDisabled();
    const explanation = acceptance.getByText("The task changed after delivery. Verify the latest result and submit an updated delivery report.");
    await expect(explanation).toBeVisible();
    await expect(explanation).not.toHaveClass(/text-destructive/);
    await expect(page.getByText("Delivery result needs to be rechecked", { exact: true })).toBeVisible();
    parent = await api.recordDeliveryEvidence(parent.id, parent.revision, { ...evidence, issue_revision: child.revision });
    await expect(accept).toBeEnabled();
    const accepted = page.waitForResponse((response) => response.request().method() === "POST" && response.url().endsWith("/delivery-review"));
    await accept.click();
    const response = await accepted;
    expect(response.status()).toBe(200);
    expect((await response.json()).status).toBe("done");
  } finally {
    await api.cleanup();
  }
});
