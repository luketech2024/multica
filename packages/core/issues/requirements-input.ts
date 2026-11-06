import type { AcceptanceCriteria } from "../types";

export function parseRequirementsInput(input: string): { criteria: AcceptanceCriteria; truncated: boolean } {
  const criteria: AcceptanceCriteria = { v: 1, version: 1, next_item_no: 1, items: [], bounds: [], gate: { spec: null, accept: null } };
  let truncated = false;
  for (const line of input.split(/\r?\n/)) {
    const text = line.trim().replace(/^(?:[-*•]\s*|\[[ xX]\]\s*|\d+[.)、](?!\d)\s*|A-\d+\s*[:：.、-]?\s*)+/i, "").trim();
    if (!text) continue;
    if (/^(不做|不涉及|本次不含)/.test(text)) {
      criteria.bounds.push(text);
      continue;
    }
    const characters = Array.from(text);
    truncated ||= characters.length > 120;
    criteria.items.push({ id: "", text: characters.slice(0, 120).join(""), origin: "user", covers: [], checked: null, note: null });
  }
  return { criteria, truncated };
}
