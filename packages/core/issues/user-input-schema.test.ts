// @vitest-environment node
import { describe, it, expect } from "vitest";
import { parseTaskUserInputs } from "./user-input-schema";

describe("user input response parsing", () => {
  it("accepts asynchronous questions without provider thread identifiers", () => {
    const rows = parseTaskUserInputs([{ id: "r", task_id: "t", issue_id: "i", status: "pending", created_at: "now", request: { origin: "platform", key: "k", questions: [{ id: "q", header: "Port", question: "Which port?", options: [{ label: "8080", description: "Test port", recommended: true }] }] }, response: null }]);
    expect(rows[0]?.request.origin).toBe("platform");
    expect(rows[0]?.request.questions[0]?.options[0]?.recommended).toBe(true);
  });
  it("keeps valid pending questions and tolerates missing options", () => {
    const rows = parseTaskUserInputs([{ id: "r", task_id: "t", issue_id: "i", status: "pending", created_at: "now", request: { key: "k", threadId: "thread", turnId: "turn", questions: [{ id: "q", header: "Port", question: "Which port?" }] }, response: null }]);
    expect(rows[0]?.request.questions[0]?.options).toEqual([]);
  });
  it("fails closed for malformed questions", () => {
    expect(parseTaskUserInputs([{ request: { questions: [] } }])).toEqual([]);
    expect(parseTaskUserInputs(null)).toEqual([]);
  });
});
