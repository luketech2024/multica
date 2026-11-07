// @vitest-environment node
import { describe, expect, it } from "vitest";
import { GitHubPullRequestSchema } from "./schemas";

const evidenceSchema = GitHubPullRequestSchema.pick({
  base_branch: true,
  head_sha: true,
  merge_commit_sha: true,
});

describe("pull request merge evidence compatibility", () => {
  it.each([{}, { base_branch: null, head_sha: "short", merge_commit_sha: 7 }])(
    "keeps missing and malformed evidence empty",
    (input) => {
      expect(evidenceSchema.parse(input)).toEqual({
        base_branch: "", head_sha: "", merge_commit_sha: "",
      });
    },
  );

  it("preserves complete SHA-1 and SHA-256 identities", () => {
    const input = { base_branch: "main", head_sha: "a".repeat(40), merge_commit_sha: "b".repeat(64) };
    expect(evidenceSchema.parse(input)).toEqual(input);
  });
});
