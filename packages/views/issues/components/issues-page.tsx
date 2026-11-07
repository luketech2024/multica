"use client";

import { ListTodo, MessageSquare, Plus } from "@multica/ui/icons";
import { Button } from "@multica/ui/components/ui/button";
import { useIssuesScope } from "@multica/core/issues/stores/issues-scope-store";
import { useStartChatWithDraft } from "../../chat/use-start-chat-with-draft";
import { useT } from "../../i18n";
import { CollectionPageState } from "../../layout/collection-page";
import { IssueSurface } from "../surface/issue-surface";
import { IssueViewHeader } from "../../issue-views/view-header";
import { useNavigation } from "../../navigation";
import { useQuery } from "@tanstack/react-query";
import {
  projectDetailOptions,
  projectListOptions,
} from "@multica/core/projects/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useRecentContextStore } from "@multica/core/chat";
import { ConfigurationSelect } from "../../projects/components/configuration-select";

const WORKSPACE_BASE = { kind: "workspace" } as const;

export function IssuesPage() {
  const { t } = useT("issues");
  const { t: tLayout } = useT("layout");
  const scope = useIssuesScope("issues");
  const startChat = useStartChatWithDraft();
  const navigation = useNavigation();
  const { searchParams } = navigation;
  const paths = useWorkspacePaths();
  const layout = searchParams.get("layout");
  const projectId = layout === "board" ? searchParams.get("project") : null;
  const wsId = useWorkspaceId();
  const project = useQuery({
    ...projectDetailOptions(wsId, projectId ?? ""),
    enabled: !!projectId,
  });
  const projects = useQuery({
    ...projectListOptions(wsId),
    enabled: layout === "board",
  });

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      {layout === "board" && (
        <div
          className="max-w-sm p-4"
          aria-busy={projects.isPending || (!!projectId && project.isPending)}
        >
          <ConfigurationSelect
            label={tLayout(($) => $.nav.projects)}
            value={projectId ?? ""}
            disabled={projects.isPending}
            items={[
              {
                value: "",
                label: tLayout(($) => $.workbench_nav.choose_project),
              },
              ...(projects.data ?? []).map((row) => ({
                value: row.id,
                label: row.title,
              })),
            ]}
            onChange={(id) => {
              const selected = projects.data?.find((row) => row.id === id);
              if (selected)
                useRecentContextStore
                  .getState()
                  .recordVisit(wsId, {
                    type: "project",
                    id,
                    label: selected.title,
                  });
              navigation.replace(paths.taskBoard(id));
            }}
          />
          {(projects.isError || project.isError) && (
            <div role="alert" className="mt-2 text-caption">
              <p>{tLayout(($) => $.workbench_nav.project_unavailable)}</p>
              <Button
                variant="outline"
                onClick={() => {
                  void projects.refetch();
                  if (projectId) void project.refetch();
                }}
              >
                {t(($) => $.pipeline_page.retry)}
              </Button>
            </div>
          )}
        </div>
      )}
      {(layout !== "board" || !!project.data) && (
        <IssueSurface
          enableWorkbenchBand
          key={`${layout ?? "default"}:${projectId ?? ""}`}
          surfaceKey={
            layout === "board"
              ? `workbench-board:${projectId ?? "all"}`
              : layout === "table"
                ? "workbench-tasks"
                : undefined
          }
          scope={
            projectId
              ? { type: "project", projectId, actorKind: scope }
              : { type: "workspace", actorKind: scope }
          }
          modes={
            layout === "board"
              ? ["board", "list"]
              : layout === "table"
                ? ["table", "list"]
                : ["board", "list", "table", "swimlane"]
          }
          batchToolbar="list"
          renderHeader={({ controller }) => (
            <IssueViewHeader
              controller={controller}
              base={
                project.data
                  ? { kind: "project", project: project.data }
                  : WORKSPACE_BASE
              }
            />
          )}
          renderEmpty={({ controller }) => (
            <CollectionPageState
              icon={ListTodo}
              title={t(($) => $.page.empty_title)}
              description={t(($) => $.page.empty_description)}
              actions={
                <>
                  <Button
                    size="sm"
                    onClick={() => controller.openCreateIssue()}
                  >
                    <Plus aria-hidden="true" />
                    {t(($) => $.views.new_issue)}
                  </Button>
                  {startChat ? (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        startChat(t(($) => $.page.break_down_prompt))
                      }
                    >
                      <MessageSquare aria-hidden="true" />
                      {t(($) => $.page.break_down_with_chat)}
                    </Button>
                  ) : null}
                </>
              }
            />
          )}
        />
      )}
    </div>
  );
}
