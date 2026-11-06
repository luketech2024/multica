// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseRequirementsInput } from "./requirements-input";

describe("requirements form input", () => {
  it("strips copied list prefixes and leaves identifiers to the server", () => {
    const { criteria } = parseRequirementsInput("* [ ] A-01 导出报表\n1. 保留筛选\nA-03：显示下载记录");
    expect(criteria.items.map((item) => item.text)).toEqual(["导出报表", "保留筛选", "显示下载记录"]);
    expect(criteria.items.every((item) => item.id === "" && item.checked === null)).toBe(true);
  });
  it("keeps boundaries without numbering them", () => {
    const { criteria } = parseRequirementsInput("不做支付\n- 不涉及退款\n本次不含订阅\n\n 导出数据");
    expect(criteria.bounds).toEqual(["不做支付", "不涉及退款", "本次不含订阅"]);
    expect(criteria.items).toHaveLength(1);
  });
  it("reports truncation and counts Unicode characters", () => {
    const { criteria, truncated } = parseRequirementsInput("字".repeat(121));
    expect(criteria.items[0]?.text).toHaveLength(120);
    expect(truncated).toBe(true);
  });
  it("preserves a decimal value at the start of a criterion", () => {
    expect(parseRequirementsInput("1.5 秒内响应").criteria.items[0]?.text).toBe("1.5 秒内响应");
  });
});
