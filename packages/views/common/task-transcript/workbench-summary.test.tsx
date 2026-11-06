// @vitest-environment jsdom
import { cleanup, fireEvent, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { WorkbenchRunSummary } from "./workbench-summary";

vi.mock("../../editor", () => ({ ReadonlyContent: ({ content }: { content: string }) => <div data-testid="full-output">{content}</div> }));
afterEach(cleanup);
describe("workbench summary disclosure", () => {
  it("shows four compact sections and lets the reader recover the full output", () => {
    const content = "Full evidence\n<!-- wb-summary v1 -->\n结论：Report ready\n验证：Tests passed\n<!-- /wb-summary -->";
    renderWithI18n(<WorkbenchRunSummary content={content} />);
    expect(screen.getByText("Report ready")).toBeInTheDocument();
    expect(screen.getAllByText("Not provided")).toHaveLength(2);
    expect(screen.queryByTestId("full-output")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Show full output" }));
    expect(screen.getByTestId("full-output")).toHaveTextContent("Full evidence");
  });
  it("renders the complete source when the marker version is unsupported", () => {
    const content = "<!-- wb-summary v2 -->结论：Future format<!-- /wb-summary -->";
    renderWithI18n(<WorkbenchRunSummary content={content} />);
    expect(screen.getByTestId("full-output")).toHaveTextContent(content);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
