// @vitest-environment node
import { describe, expect, it } from "vitest";
import { ExternalTicketCountSchema, IssueSchema } from "./schemas";

describe("external ticket response compatibility", () => {
  const schema = IssueSchema.pick({ external_ticket_no: true });
  it("reads old responses and rejects malformed external identifiers", () => {
    expect(schema.parse({})).toEqual({});
    expect(
      schema.parse({ external_ticket_no: "XQ-3482" }).external_ticket_no,
    ).toBe("XQ-3482");
    expect(
      schema.parse({ external_ticket_no: 3482 }).external_ticket_no,
    ).toBeUndefined();
    expect(
      schema.parse({ external_ticket_no: "bad space" }).external_ticket_no,
    ).toBeUndefined();
  });
  it("requires a nonnegative integer for duplicate hints", () => {
    expect(ExternalTicketCountSchema.safeParse({ count: "2" }).success).toBe(
      false,
    );
    expect(ExternalTicketCountSchema.safeParse({ count: -1 }).success).toBe(
      false,
    );
  });
});
