export interface WorkbenchSummary {
  conclusion: string | null;
  changes: string[];
  validation: string[];
  risks: string[];
}

// Only a paired, supported marker grants permission to show a summary.
export function parseWorkbenchSummary(content: string): WorkbenchSummary | null {
  const start = /<!--\s*wb-summary(?:\s+(\S+))?\s*-->/.exec(content);
  if (!start || (start[1] && start[1] !== "v1")) return null;
  const remainder = content.slice(start.index + start[0].length);
  const end = /<!--\s*\/wb-summary\s*-->/.exec(remainder);
  if (!end) return null;
  const body = remainder.slice(0, end.index);
  if (/<!--\s*wb-summary/.test(body)) return null;
  const result: WorkbenchSummary = { conclusion: null, changes: [], validation: [], risks: [] };
  const sections = { "结论": "conclusion", "改动": "changes", "验证": "validation", "风险": "risks" } as const;
  let section: keyof WorkbenchSummary | undefined;
  for (const line of body.split(/\r?\n/)) {
    const heading = /^(结论|改动|验证|风险)\s*[:：]\s*(.*)$/.exec(line.trim());
    if (heading) section = sections[heading[1] as keyof typeof sections];
    const text = (heading ? heading[2]! : line).trim().replace(/^[-*]\s+/, "");
    if (!text || !section) continue;
    if (section === "conclusion") {
      result.conclusion ??= clip(text, 80);
    } else if (result[section].length < 5) {
      result[section].push(clip(text, 100));
    }
  }
  return result.conclusion || result.changes.length || result.validation.length || result.risks.length ? result : null;
}

function clip(text: string, limit: number): string {
  const chars = Array.from(text);
  return chars.length > limit ? chars.slice(0, limit - 1).join("") + "…" : text;
}
