// @vitest-environment node
import { describe, expect, it } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { IssueSchema } from "../api/schemas";
import type { Issue, ListIssuesCache } from "../types";
import { issueKeys } from "./queries";
import { onIssueCreated, onIssueUpdated } from "./ws-updaters";

const issue: Issue = {
  id: "issue-1",
  workspace_id: "ws-1",
  number: 1,
  identifier: "TEST-1",
  title: "Static website",
  description: null,
  status: "todo",
  priority: "none",
  assignee_type: null,
  assignee_id: null,
  creator_type: "member",
  creator_id: "user-1",
  parent_issue_id: null,
  project_id: null,
  position: 0,
  stage: null,
  start_date: null,
  due_date: null,
  metadata: {},
  properties: {},
  labels: [],
  created_at: "2026-11-09T00:00:00Z",
  updated_at: "2026-11-09T00:00:00Z",
  revision: 1,
};

const emptyCriteria = {
  v: 1,
  version: 1,
  next_item_no: 1,
  items: [],
  bounds: [],
  gate: { spec: null, accept: null },
};
const confirmedCriteria = {
  ...emptyCriteria,
  gate: {
    spec: { passed_at: "2026-11-09T00:00:00Z", passed_by: "user-1" },
    accept: null,
  },
};

describe("realtime issue requirements", () => {
  it.each([
    { name: "legacy array", raw: [], expected: emptyCriteria },
    { name: "legacy null", raw: null, expected: emptyCriteria },
    { name: "omitted gates", raw: { v: 1, items: [] }, expected: emptyCriteria },
    {
      name: "confirmed requirements",
      raw: confirmedCriteria,
      expected: confirmedCriteria,
    },
    {
      name: "malformed requirements",
      raw: { v: 1, items: [], gate: { spec: true } },
      expected: undefined,
    },
  ])("normalizes $name before creating or updating cached issues", ({ raw, expected }) => {
    const qc = new QueryClient();
    const listKey = issueKeys.list(issue.workspace_id);
    const detailKey = issueKeys.detail(issue.workspace_id, issue.id);
    const childrenKey = issueKeys.children(issue.workspace_id, "parent-1");
    qc.setQueryData<ListIssuesCache>(listKey, {
      byStatus: { unstarted: { issues: [], total: 0 } },
    });
    // WebSocket frames carry the wire value before API schema normalization.
    const incoming = { ...issue, acceptance_criteria: raw } as unknown as Issue;
    onIssueCreated(qc, issue.workspace_id, incoming);
    expect(
      qc.getQueryData<ListIssuesCache>(listKey)?.byStatus.unstarted?.issues[0]?.acceptance_criteria,
    ).toEqual(expected);

    const loaded = {
      ...IssueSchema.parse({ ...issue, acceptance_criteria: raw }),
      parent_issue_id: "parent-1",
    } as Issue;
    qc.setQueryData(detailKey, loaded);
    qc.setQueryData(childrenKey, [loaded]);
    onIssueUpdated(qc, issue.workspace_id, {
      ...incoming,
      parent_issue_id: "parent-1",
      revision: 2,
    });
    expect(qc.getQueryData<Issue>(detailKey)?.acceptance_criteria).toEqual(expected);
    expect(qc.getQueryData<Issue[]>(childrenKey)?.[0]?.acceptance_criteria).toEqual(expected);
    expect(
      qc.getQueryData<ListIssuesCache>(listKey)?.byStatus.unstarted?.issues[0]?.acceptance_criteria,
    ).toEqual(expected);
    qc.clear();
  });

  it("preserves confirmed requirements when a partial event omits them", () => {
    const qc = new QueryClient();
    const detailKey = issueKeys.detail(issue.workspace_id, issue.id);
    qc.setQueryData(detailKey, { ...issue, acceptance_criteria: confirmedCriteria });
    onIssueUpdated(qc, issue.workspace_id, {
      id: issue.id,
      title: "Updated title",
      revision: 2,
    });
    expect(qc.getQueryData<Issue>(detailKey)).toMatchObject({
      title: "Updated title",
      acceptance_criteria: confirmedCriteria,
    });
    qc.clear();
  });
});
