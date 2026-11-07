"use client";

import { forwardRef, useCallback, useEffect, useMemo, useRef } from "react";
import { VirtuosoGrid, type VirtuosoGridHandle } from "react-virtuoso";
import type { Issue, Project } from "@multica/core/types";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../../navigation";
import { useRestoredScrollOffset } from "../../platform";
import { useT } from "../../i18n";
import { IssueActionsContextMenu } from "../actions";
import { useIssuePeekActions, useIssuePeekLinkProps, useIsIssuePeeked, PEEK_TARGET_ATTR } from "../surface/peek-context";
import type { IssueStatusPagination } from "../surface/use-issue-status-branches";
import { WorkbenchTaskCardContent } from "./workbench-task-card";
import { useWorkbenchKeyboardOrder, useWorkbenchKeyboardReveal, useWorkbenchKeyboardTarget } from "../surface/workbench-keyboard";
import { ListLoadMoreFooter } from "./list-load-more-footer";

const GridList = forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(function GridList({ className, ...props }, ref) {
  return <div {...props} ref={ref} className={`grid grid-cols-2 gap-3 p-4 min-[1280px]:grid-cols-3 min-[1600px]:grid-cols-4 ${className ?? ""}`} />;
});

function FlowCard({ issue, project }: { issue: Issue; project?: Project }) {
  const paths = useWorkspacePaths();
  const keyboard = useWorkbenchKeyboardTarget(issue.id);
  const peekProps = useIssuePeekLinkProps(issue.id);
  const peeked = useIsIssuePeeked(issue.id);
  return (
    <IssueActionsContextMenu issue={issue}>
      <AppLink
        href={paths.issueDetail(issue.id)}
        newTabTitle={issue.identifier}
        {...peekProps}
        {...keyboard}
        {...{ [PEEK_TARGET_ATTR]: issue.id }}
        data-peeked={peeked ? "" : undefined}
        className="group/card block min-w-0 rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring data-[keyboard-selected]:ring-2 data-[keyboard-selected]:ring-ring"
      >
        <WorkbenchTaskCardContent issue={issue} project={project} />
      </AppLink>
    </IssueActionsContextMenu>
  );
}

export function WorkbenchCardFlow({ issues, projectMap, statusPagination }: {
  issues: Issue[];
  projectMap: Map<string, Project>;
  statusPagination: IssueStatusPagination;
}) {
  const { t } = useT("issues");
  const peek = useIssuePeekActions();
  const gridRef = useRef<VirtuosoGridHandle>(null);
  const restoredScrollTop = useRestoredScrollOffset("workbench-card-flow");
  const ids = useMemo(() => issues.map((issue) => issue.id), [issues]);
  useWorkbenchKeyboardOrder(ids);
  useWorkbenchKeyboardReveal(useCallback((id: string) => {
    const index = ids.indexOf(id);
    if (index < 0) return false;
    gridRef.current?.scrollToIndex({ index, align: "center" });
    return true;
  }, [ids]));
  useEffect(() => {
    peek?.publishColumns([issues.map((issue) => issue.id)]);
    return () => peek?.publishColumns(null);
  }, [issues, peek]);
  const components = useMemo(() => ({
    List: GridList,
    Footer: () => (
      <div className="px-4 pb-4">
        {Object.entries(statusPagination).map(([status, page]) => (
          <ListLoadMoreFooter
            key={status}
            total={page.total}
            hasMore={page.hasMore}
            isLoading={page.isLoading || page.isFetching}
            isError={page.isError}
            onLoadMore={page.loadMore}
            onRetry={page.retry}
          />
        ))}
      </div>
    ),
  }), [statusPagination]);

  return (
    <section aria-label={t(($) => $.workbench_card.flow)} className="flex min-h-0 flex-1 flex-col">
      <VirtuosoGrid
        ref={gridRef}
        data={issues}
        style={{ height: "100%" }}
        data-tab-scroll-root="workbench-card-flow"
        initialTopMostItemIndex={{ index: 0, offset: restoredScrollTop ?? 0 }}
        initialItemCount={12}
        increaseViewportBy={200}
        components={components}
        computeItemKey={(_index, issue) => issue.id}
        itemContent={(_index, issue) => (
          <FlowCard issue={issue} project={issue.project_id ? projectMap.get(issue.project_id) : undefined} />
        )}
      />
    </section>
  );
}
