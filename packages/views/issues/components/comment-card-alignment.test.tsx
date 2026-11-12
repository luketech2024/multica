import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { forwardRef, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { TimelineEntry } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";

// Message alignment follows the author independently of the composer context.

vi.mock("@multica/core/api", () => ({
  api: { uploadFile: vi.fn() },
  dispatchReasonCode: () => undefined,
  errorCode: () => undefined,
}));

vi.mock("../../navigation", () => ({
  useNavigation: () => ({
    push: vi.fn(),
    pathname: "/acme/issues",
    getShareableUrl: (p: string) => `https://app.example${p}`,
  }),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: () => "Ada" }),
}));

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: () => null,
}));

vi.mock("../hooks/use-comment-trigger-preview", () => ({
  useCommentTriggerPreview: () => ({ agents: [], blocked: [] }),
}));

vi.mock("../../editor", async () => ({
  ...(await vi.importActual<typeof import("../../editor/use-upload-gate")>("../../editor/use-upload-gate")),
  ...(await vi.importActual<typeof import("../../editor/use-lazy-editor")>("../../editor/use-lazy-editor")),
  ...(await vi.importActual<typeof import("../../editor/use-composer-submit")>("../../editor/use-composer-submit")),
  useEditorUpload: () => ({ uploadWithToast: vi.fn(), upload: vi.fn(), uploading: false }),
  useFileDropZone: () => ({ isDragOver: false, dropZoneProps: {} }),
  FileDropOverlay: () => null,
  ReadonlyContent: ({ content }: { content: string }) => <div>{content}</div>,
  Attachment: () => null,
  AttachmentDownloadProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
  ContentEditor: forwardRef(function MockContentEditor() {
    return <textarea data-testid="editor" />;
  }),
}));

import { CommentCard } from "./comment-card";

function comment(
  id: string,
  parentId: string | null,
  author: Pick<TimelineEntry, "actor_type" | "actor_id"> = { actor_type: "member", actor_id: "user-1" },
): TimelineEntry {
  return {
    type: "comment",
    id,
    ...author,
    content: `body ${id}`,
    parent_id: parentId,
    comment_type: "comment",
    reactions: [],
    attachments: [],
    created_at: "2026-09-11T07:00:00Z",
    updated_at: "2026-09-11T07:00:00Z",
    revision: 1,
  };
}

function renderThread(
  root: TimelineEntry,
  replies: TimelineEntry[],
  { resolvable = true }: { resolvable?: boolean } = {},
) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderWithI18n(
    <QueryClientProvider client={qc}>
      <CommentCard
        issueId="issue-1"
        entry={root}
        replies={replies}
        currentUserId="user-1"
        onReply={vi.fn().mockResolvedValue(true)}
        onEdit={vi.fn().mockResolvedValue(undefined)}
        onDelete={vi.fn()}
        onToggleReaction={vi.fn()}
        onCopyLink={vi.fn()}
        onCreateSubIssue={vi.fn()}
        onResolveToggle={resolvable ? vi.fn() : undefined}
      />
    </QueryClientProvider>,
  );
}

describe("comment author alignment", () => {
  it.each([
    ["member", "user-1", "right"],
    ["member", "another-user", "left"],
    ["agent", "agent-1", "left"],
    ["agent", "user-1", "left"],
  ] as const)("places %s %s on the %s", (actor_type, actor_id, side) => {
    renderThread(comment("root", null, { actor_type, actor_id }), []);
    expect(screen.getByText("body root").closest("[data-comment-block]"))
      .toHaveAttribute("data-message-side", side);
  });

  it("aligns each reply by its author independently of the root author", () => {
    renderThread(comment("root", null, { actor_type: "agent", actor_id: "agent-1" }), [
      comment("own", "root"),
      comment("other", "root", { actor_type: "member", actor_id: "another-user" }),
      comment("agent", "root", { actor_type: "agent", actor_id: "agent-1" }),
    ]);
    for (const [id, side] of [["root", "left"], ["own", "right"], ["other", "left"], ["agent", "left"]]) {
      expect(screen.getByText(`body ${id}`).closest("[data-comment-block]"))
        .toHaveAttribute("data-message-side", side);
    }
  });
});
