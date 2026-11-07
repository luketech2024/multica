"use client";

import { useQuery } from "@tanstack/react-query";
import { useChatStore, useRecentContextStore } from "@multica/core/chat";
import { chatSessionsOptions } from "@multica/core/chat/queries";
import { countUnreadChatMessages } from "@multica/core/chat/unread";
import { CappedNumberFlow } from "@multica/ui/components/ui/number-flow";
import { Pin, PinOff, LayoutDashboard } from "@multica/ui/icons";
import { useInboxUnreadCount } from "@multica/core/inbox/queries";
import { useCurrentWorkspace, useWorkspacePaths } from "@multica/core/paths";
import {
  WORKBENCH_NAVIGATION,
  useWorkbenchNavigationStore,
  type WorkbenchNavID,
} from "@multica/core/workbench/navigation";
import { useSidebar } from "@multica/ui/components/ui/sidebar";
import { TooltipHint } from "@multica/ui/components/common/tooltip-hint";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { AppLink, useNavigation } from "../navigation";
import { WhatsNewLauncher } from "../whats-new/whats-new-launcher";
import { AccountMenu } from "./account-menu";
import { NewIssueButton, SearchButton } from "./global-actions";
import { HelpLauncher } from "./help-launcher";
import { isPathWithin } from "./panel-nav";
import { RAIL_ICON_BUTTON_CLASS } from "./rail-styles";
import { RailWorkspaces } from "./rail-workspaces";
import { routeIconForPath } from "./route-icon-components";
import { useTeamAgentStatus } from "./panels/team-status";
import { useAppForeground } from "../common/use-app-foreground";

export function AppRail({
  topSlot,
  globalActions = true,
  className,
}: {
  topSlot?: React.ReactNode;
  globalActions?: boolean;
  className?: string;
}) {
  const { t } = useT("layout");
  const paths = useWorkspacePaths();
  const workspace = useCurrentWorkspace();
  const { pathname, searchParams } = useNavigation();
  const { open, isCompact } = useSidebar();
  const expanded = open || isCompact;
  const pinned = useWorkbenchNavigationStore((state) => state.pinned);
  const togglePin = useWorkbenchNavigationStore((state) => state.togglePin);
  const lastProject = useRecentContextStore(
    (state) =>
      state.byWorkspace[workspace?.id ?? ""]?.find(
        (entry) => entry.type === "project",
      )?.id ?? "",
  );
  const unread = useInboxUnreadCount(workspace?.id);
  const { data: sessions = [] } = useQuery({
    ...chatSessionsOptions(workspace?.id ?? ""),
    enabled: !!workspace?.id,
  });
  const activeSession = useChatStore((state) => state.activeSessionId);
  const chatOpen = useChatStore((state) => state.isOpen);
  const foreground = useAppForeground();
  const chatUnread = countUnreadChatMessages(
    sessions,
    foreground && (chatOpen || isPathWithin(pathname, paths.chat()))
      ? activeSession
      : null,
  );
  const { stuck } = useTeamAgentStatus(workspace?.id);
  const projectPrefix = paths.projects() + "/";
  const projectId = pathname.startsWith(projectPrefix)
    ? decodeURIComponent(
        pathname.slice(projectPrefix.length).split("/")[0] ?? "",
      )
    : "";
  const hrefs: Record<WorkbenchNavID, string> = {
    inbox: paths.inbox(),
    my_issues: paths.myIssues(),
    issues: paths.taskList(),
    chat: paths.chat(),
    projects: paths.projects(),
    board: paths.taskBoard(projectId || lastProject),
    releases: paths.releases(),
    agents: paths.agents(),
    squads: paths.squads(),
    skills: paths.skills(),
    runtimes: paths.runtimes(),
    security: paths.security(),
    usage: paths.usage(),
    settings: paths.settings(),
  };
  const groups = ["pinned", "personal", "work", "team", "system"] as const;
  const active = (id: WorkbenchNavID) => {
    if (id === "board")
      return (
        pathname === paths.issues() && searchParams.get("layout") === "board"
      );
    if (id === "issues")
      return (
        isPathWithin(pathname, paths.issues()) &&
        searchParams.get("layout") !== "board"
      );
    return isPathWithin(pathname, hrefs[id]);
  };
  return (
    <nav
      aria-label={t(($) => $.rail.label)}
      data-slot="app-rail"
      data-expanded={expanded}
      className={cn(
        "flex h-full shrink-0 flex-col overflow-y-auto overflow-x-hidden border-r border-sidebar-border bg-sidebar px-2 py-3 text-sidebar-foreground",
        expanded ? "w-[240px]" : "w-16",
        className,
      )}
    >
      {topSlot}
      <div className="flex items-center gap-2 px-2">
        <RailWorkspaces />
        {expanded && (
          <span className="min-w-0 truncate text-label font-medium">
            {workspace?.name}
          </span>
        )}
      </div>
      {globalActions && (
        <div
          className={cn(
            "my-2 flex items-center gap-1 px-1",
            expanded ? "flex-row" : "flex-col",
          )}
        >
          <NewIssueButton
            className={RAIL_ICON_BUTTON_CLASS}
            tooltipSide="right"
          />
          <SearchButton
            className={RAIL_ICON_BUTTON_CLASS}
            tooltipSide="right"
          />
        </div>
      )}
      <div className="flex-1 space-y-3 py-2" data-workbench-navigation>
        {groups.map((group) => {
          const entries =
            group === "pinned"
              ? pinned.map(
                  (id) =>
                    WORKBENCH_NAVIGATION.find((entry) => entry.id === id)!,
                )
              : WORKBENCH_NAVIGATION.filter(
                  (entry) =>
                    entry.group === group && !pinned.includes(entry.id),
                );
          if (entries.length === 0) return null;
          return (
            <section
              key={group}
              className={group === "pinned" ? "border-b border-sidebar-border pb-3" : undefined}
              aria-label={t(($) => $.workbench_nav.groups[group])}
            >
              {expanded && (
                <h2 className="px-2 pb-1 text-micro text-muted-foreground">
                  {t(($) => $.workbench_nav.groups[group])}
                </h2>
              )}
              <ul className="space-y-0.5">
                {entries.map((entry) => {
                  const selected = active(entry.id);
                  const isPinned = pinned.includes(entry.id);
                  const full = !isPinned && pinned.length >= 3;
                  const count =
                    entry.id === "inbox"
                      ? unread
                      : entry.id === "chat"
                        ? chatUnread
                        : 0;
                  const label =
                    entry.id === "board"
                      ? t(($) => $.workbench_nav.board)
                      : entry.id === "chat"
                        ? t(($) => $.workbench_nav.chat)
                        : t(($) => $.nav[entry.id]);
                  const Icon =
                    entry.id === "board"
                      ? LayoutDashboard
                      : routeIconForPath(hrefs[entry.id].split("?")[0]!);
                  return (
                    <li key={entry.id} className="group relative">
                      <TooltipHint label={label} side="right">
                        <AppLink
                          href={hrefs[entry.id]}
                          aria-label={label}
                          aria-current={selected ? "page" : undefined}
                          className={cn(
                            "flex h-8 items-center gap-2 rounded-lg px-2 text-muted-foreground outline-none hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-ring",
                            selected &&
                              "bg-sidebar-accent font-medium text-sidebar-accent-foreground hover:bg-sidebar-accent",
                            expanded && "pr-8",
                          )}
                        >
                          <Icon className="size-4 shrink-0" />
                          {expanded && (
                            <span className="min-w-0 flex-1 truncate text-label">
                              {label}
                            </span>
                          )}
                          {count > 0 && (
                            <CappedNumberFlow
                              value={count}
                              animated={false}
                              className="rounded-full bg-brand px-1 text-micro text-brand-foreground"
                            />
                          )}
                          {entry.id === "agents" && stuck.length > 0 && (
                            <span
                              aria-hidden
                              className="size-2 shrink-0 rounded-full bg-warning"
                            />
                          )}
                        </AppLink>
                      </TooltipHint>
                      {expanded && (
                        <button
                          type="button"
                          disabled={full}
                          title={
                            full
                              ? t(($) => $.workbench_nav.pin_limit)
                              : undefined
                          }
                          aria-label={t(
                            ($) =>
                              isPinned
                                ? $.workbench_nav.unpin
                                : $.workbench_nav.pin,
                            { name: label },
                          )}
                          onClick={() => togglePin(entry.id)}
                          className="absolute top-1 right-1 rounded p-1 text-muted-foreground opacity-0 group-hover:opacity-100 focus-visible:opacity-100 disabled:cursor-not-allowed"
                        >
                          {isPinned ? (
                            <PinOff className="size-3" />
                          ) : (
                            <Pin className="size-3" />
                          )}
                        </button>
                      )}
                    </li>
                  );
                })}
              </ul>
            </section>
          );
        })}
      </div>
      <div
        className={cn(
          "flex items-center gap-1 border-t border-sidebar-border px-1 pt-3",
          expanded ? "flex-row" : "flex-col",
        )}
      >
        <WhatsNewLauncher />
        <HelpLauncher />
        <AccountMenu />
      </div>
    </nav>
  );
}
