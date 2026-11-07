"use client";

import { useQuery } from "@tanstack/react-query";
import type { Issue } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { releaseReadinessOptions } from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";

export function ReleaseReadiness({ issue }: { issue: Issue }) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const review = useQuery(releaseReadinessOptions(wsId, issue.id, issue.revision));
  return <section className="space-y-2" aria-label={t(($) => $.release_readiness.title)}>
    <h4 className="text-label font-medium">{t(($) => $.release_readiness.title)}</h4>
    {review.isPending && <p role="status" className="text-caption">{t(($) => $.delivery_review.loading)}</p>}
    {review.isError && <div role="alert" className="space-y-2 text-caption">
      <p>{t(($) => $.release_readiness.failed)}</p>
      <Button size="sm" variant="outline" onClick={() => void review.refetch()}>{t(($) => $.release_readiness.retry)}</Button>
    </div>}
    {review.data && <>
      {review.data.blockers.length === 0 && <p className="text-caption text-success">{t(($) => $.release_readiness.matched)}</p>}
      {review.data.blockers.length === 0 && issue.project_id && <AppLink className="text-caption text-brand underline" href={`${paths.releases()}?project=${encodeURIComponent(issue.project_id)}`}>{t(($) => $.release_readiness.join)}</AppLink>}
      <ul className="space-y-1 text-caption text-destructive">
        {review.data.blockers.map((blocker, index) => <li key={`${blocker.repo_id}-${index}`}>{t(($) => $.release_readiness.blockers[blocker.code])}</li>)}
      </ul>
      <ul className="space-y-2 text-caption">
        {review.data.merges.map((merge) => <li key={merge.repo_id} className="min-w-0">
          <a className="block truncate text-brand underline" href={merge.url} target="_blank" rel="noopener noreferrer">{merge.url}</a>
          <p className="truncate text-muted-foreground" title={merge.merge_commit}>{merge.base_branch} · <code>{merge.merge_commit.slice(0, 12)}</code></p>
        </li>)}
      </ul>
    </>}
  </section>;
}
