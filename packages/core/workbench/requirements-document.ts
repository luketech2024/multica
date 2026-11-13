export function requirementSection(content: string, section: string): string | undefined {
  const lines = content.split("\n");
  const heading = section.replace(/^#+\s*/, "").trim();
  const start = lines.findIndex((line) => /^#{1,6}\s/.test(line) && line.replace(/^#+\s*/, "").trim() === heading);
  if (start < 0) return undefined;
  const level = lines[start]!.match(/^#+/)![0].length;
  const next = lines.findIndex((line, index) => index > start && /^#{1,6}\s/.test(line) && line.match(/^#+/)![0].length <= level);
  return lines.slice(start, next < 0 ? undefined : next).join("\n");
}

export function requirementChanges(previous: string, current: string) {
  if (previous === current) return null;
  const before = previous.split("\n");
  const after = current.split("\n");
  let start = 0;
  while (start < before.length && start < after.length && before[start] === after[start]) start++;
  let endBefore = before.length;
  let endAfter = after.length;
  while (endBefore > start && endAfter > start && before[endBefore - 1] === after[endAfter - 1]) {
    endBefore--;
    endAfter--;
  }
  return { before: before.slice(start, endBefore).join("\n"), after: after.slice(start, endAfter).join("\n") };
}
