"use client";

import { useEffect, useRef } from "react";
import { useCanonicalIssue } from "@multica/core/issues/canonical-id";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useNavigation } from "../../navigation";
import { IssueDetail, IssueDetailSkeleton, IssueNotFound } from "./issue-detail";
import { IssuePeekHost } from "./issue-peek";

interface IssueDetailRouteProps {
  /**
   * Raw `/{ws}/issues/{segment}` parameter. Either a UUID (older links, and
   * anything the app itself linked before the issue row was in hand) or a
   * human-readable identifier such as `MUL-123`.
   */
  routeId: string;
  onDelete?: () => void;
}

/**
 * Native issue links use their resolved identifier. Workbench tasks pass an
 * immutable ID so workspace prefix changes do not invalidate their links.
 *
 * A replace, not a push: the UUID URL is the same page, and a history entry
 * for it would make Back bounce the user between two spellings of one issue.
 */
export function useCanonicalIssueUrl(
  routeId: string,
  identifier: string | undefined,
  hash = "",
  immutableId?: string,
) {
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const segment = immutableId ?? identifier;
  const canonicalHref = segment ? `${paths.issueDetail(segment)}${hash}` : null;
  // `useWorkspacePaths()` and the navigation adapter are both rebuilt on
  // render, so this ref — not the dependency array — is what guarantees the
  // replace runs once per target instead of on every commit.
  const replacedRef = useRef<string | null>(null);

  useEffect(() => {
    if (!canonicalHref || routeId === segment) return;
    if (replacedRef.current === canonicalHref) return;
    replacedRef.current = canonicalHref;
    navigation.replace(canonicalHref);
  }, [canonicalHref, segment, routeId, navigation]);
}

export function parseCommentHighlightHash(hash: string): string | undefined {
  const match = /^#comment-([A-Za-z0-9_-]+)$/.exec(hash);
  return match?.[1];
}

function useCommentHighlightHash(): { hash: string; commentId?: string } {
  const { hash } = useNavigation();

  return { hash, commentId: parseCommentHighlightHash(hash) };
}

/**
 * Route-level wrapper around `IssueDetail` for `/{ws}/issues/{id}`.
 *
 * Two jobs the panel-embedded `IssueDetail` must not do:
 *  - resolve an identifier segment to the issue's UUID before rendering, so
 *    every cache key below stays UUID-keyed and realtime updates land (see
 *    `useCanonicalIssue`);
 *  - rewrite the address bar to the canonical identifier URL. That belongs to
 *    the route and only the route — the inbox renders `IssueDetail` in a side
 *    panel, where replacing the URL would navigate the user out of the inbox.
 */
export function IssueDetailRoute({ routeId, onDelete }: IssueDetailRouteProps) {
  const wsId = useWorkspaceId();
  const { canonicalId, issue, isResolving, notFound } = useCanonicalIssue(wsId, routeId);
  const highlight = useCommentHighlightHash();

  // Legacy project tasks need a stable URL before acceptance recovery creates
  // their team snapshot, or canonical navigation can discard an open form.
  const immutableId = issue?.project_id || issue?.acceptance_criteria?.team || issue?.workbench_handoff
    ? canonicalId ?? undefined
    : undefined;
  useCanonicalIssueUrl(routeId, issue?.identifier, highlight.hash, immutableId);

  if (isResolving) return <IssueDetailSkeleton />;

  // Render not-found here rather than handing the unresolved segment down.
  // `IssueDetail` would mount a second observer on the query that just failed,
  // refetch it, and restart this component's resolve/remount cycle — an
  // unbounded request loop that never settles. See `CanonicalIssue.notFound`.
  if (notFound || !canonicalId) return <IssueNotFound showBackLink={!onDelete} />;

  // Finish the workbench's stable-ID redirect before exposing editable drafts.
  if (immutableId && routeId !== immutableId) return <IssueDetailSkeleton />;

  // The page hosts a side peek so Shift+Click on an issue mention in the
  // description or comments opens it floating over the page. Keyed so moving
  // to another issue closes the peek.
  return (
    <IssuePeekHost key={canonicalId}>
      <IssueDetail
        issueId={canonicalId}
        onDelete={onDelete}
        highlightCommentId={highlight.commentId}
      />
    </IssuePeekHost>
  );
}
