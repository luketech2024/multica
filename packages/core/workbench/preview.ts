export const previewPropertyName = "preview_url";

export function previewPath(projectId: string, identifier: string): string {
  return `/preview/${encodeURIComponent(projectId)}/${encodeURIComponent(identifier)}/`;
}

export function previewURL(
  origin: string,
  projectId: string,
  identifier: string,
): string | null {
  try {
    const parsed = new URL(
      origin.includes("://") ? origin : `https://${origin}`,
    );
    if (
      !origin.trim() ||
      !["http:", "https:"].includes(parsed.protocol) ||
      parsed.username ||
      parsed.password ||
      parsed.pathname !== "/" ||
      parsed.search ||
      parsed.hash
    )
      return null;
    return new URL(previewPath(projectId, identifier), parsed.origin).href;
  } catch {
    return null;
  }
}

export function validPreviewURL(value: string): boolean {
  try {
    const parsed = new URL(value);
    return (
      ["http:", "https:"].includes(parsed.protocol) &&
      !!parsed.hostname &&
      !parsed.username &&
      !parsed.password
    );
  } catch {
    return false;
  }
}
