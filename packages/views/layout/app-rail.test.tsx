import type { ReactNode } from "react";
import { fireEvent, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SidebarProvider, useSidebar } from "@multica/ui/components/ui/sidebar";
import { renderWithI18n } from "../test/i18n";
import { AppRail } from "./app-rail";
import { useWorkbenchNavigationStore } from "@multica/core/workbench/navigation";
import { AppShellProvider } from "./app-shell-context";

const state = vi.hoisted(() => ({
  pathname: "/acme/issues",
  search: "",
  appForeground: true,
  inboxUnread: 0,
  chatSessions: [] as { id: string; unread_count?: number }[],
  chat: { activeSessionId: null as string | null, isOpen: false },
  stuck: [] as unknown[],
}));

vi.mock("../navigation", () => ({
  AppLink: ({
    children,
    href,
    newTabTitle: _newTabTitle,
    ...props
  }: {
    children: ReactNode;
    href: string;
    newTabTitle?: string;
  } & React.ComponentProps<"a">) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
  useNavigation: () => ({
    pathname: state.pathname,
    searchParams: new URLSearchParams(state.search),
    push: vi.fn(),
  }),
}));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return {
    ...actual,
    useCurrentWorkspace: () => ({ id: "ws-1", name: "Acme", slug: "acme" }),
    useWorkspacePaths: () => actual.paths.workspace("acme"),
  };
});
vi.mock("@multica/core/inbox/queries", () => ({
  useInboxUnreadCount: () => state.inboxUnread,
}));
vi.mock("@multica/core/chat/queries", () => ({
  chatSessionsOptions: (wsId: string) => ({
    queryKey: ["chat", wsId, "sessions"],
    queryFn: async () => state.chatSessions,
  }),
}));
// Callable-store shape (selectorFn + getState) per the repo testing rules.
vi.mock("@multica/core/chat", () => ({
  useRecentContextStore: Object.assign(
    (selector: (s: { byWorkspace: Record<string, never[]> }) => unknown) =>
      selector({ byWorkspace: {} }),
    { getState: () => ({ byWorkspace: {} }) },
  ),
  useChatStore: Object.assign(
    (selector: (s: typeof state.chat) => unknown) => selector(state.chat),
    { getState: () => state.chat },
  ),
}));
vi.mock("@multica/core/shortcuts", () => ({ useShortcut: () => null }));
vi.mock("../common/use-app-foreground", () => ({
  useAppForeground: () => state.appForeground,
}));
vi.mock("./panels/team-status", () => ({
  useTeamAgentStatus: () => ({ stuck: state.stuck }),
}));
vi.mock("./rail-workspaces", () => ({ RailWorkspaces: () => null }));
vi.mock("./help-launcher", () => ({ HelpLauncher: () => null }));
vi.mock("../whats-new/whats-new-launcher", () => ({
  WhatsNewLauncher: () => null,
}));
vi.mock("./account-menu", () => ({ AccountMenu: () => null }));
vi.mock("@multica/ui/components/common/tooltip-hint", () => ({
  TooltipHint: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

function CollapseButton() {
  const { setOpen } = useSidebar();
  return (
    <button type="button" onClick={() => setOpen(false)}>
      collapse
    </button>
  );
}

function renderRail(children?: ReactNode, rail: ReactNode = <AppRail />) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
  client.setQueryData(["chat", "ws-1", "sessions"], state.chatSessions);
  return renderWithI18n(
    <QueryClientProvider client={client}>
      <SidebarProvider>
        <AppShellProvider>
          {rail}
          <CollapseButton />
          {children}
        </AppShellProvider>
      </SidebarProvider>
    </QueryClientProvider>,
  );
}

const link = (href: string) =>
  document.querySelector<HTMLAnchorElement>(`a[href="${href}"]`);
const countBadge = (href: string) =>
  link(href)?.querySelector("number-flow-react") ?? null;

beforeEach(() => {
  state.pathname = "/acme/issues";
  state.search = "";
  useWorkbenchNavigationStore.setState({
    pinned: ["inbox", "my_issues", "issues"],
  });
  state.appForeground = true;
  state.inboxUnread = 0;
  state.chatSessions = [];
  state.chat = { activeSessionId: null, isOpen: false };
  state.stuck = [];
});

describe("AppRail global actions", () => {
  it("carries New issue and search by default", () => {
    renderRail();
    expect(screen.getByRole("button", { name: /^new issue$/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Search..." })).toBeTruthy();
  });

  it("leaves them to the desktop title bar when turned off", () => {
    renderRail(undefined, <AppRail globalActions={false} />);
    expect(screen.queryByRole("button", { name: /^new issue$/i })).toBeNull();
    expect(screen.queryByRole("button", { name: "Search..." })).toBeNull();
  });
});

describe("AppRail navigation", () => {
  it("shows fourteen destinations and highlights the precise section", () => {
    state.pathname = "/acme/projects/p-1";
    renderRail();
    const navigation = document.querySelector("[data-workbench-navigation]")!;
    expect(navigation.querySelectorAll("a")).toHaveLength(14);
    expect(link("/acme/autopilots")).toBeNull();
    expect(link("/acme/projects")).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Task board" })).toHaveAttribute(
      "href",
      "/acme/issues?layout=board&project=p-1",
    );
    expect(
      screen.getByRole("link", { name: "Task board" }),
    ).not.toHaveAttribute("aria-current");
  });

  it("moves pinned destinations without duplicating them and enforces three pins", () => {
    renderRail();
    expect(screen.getByRole("button", { name: "Pin Projects" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Unpin Inbox" }));
    fireEvent.click(screen.getByRole("button", { name: "Pin Projects" }));
    const pinned = screen.getByRole("region", { name: "Pinned" });
    expect(
      within(pinned).getByRole("link", { name: "Projects" }),
    ).toBeVisible();
    expect(within(pinned).queryByRole("link", { name: "Inbox" })).toBeNull();
    expect(
      document.querySelectorAll("[data-workbench-navigation] a"),
    ).toHaveLength(14);
    expect(useWorkbenchNavigationStore.getState().pinned).toEqual([
      "my_issues",
      "issues",
      "projects",
    ]);
  });

  it("distinguishes the task board from the task list on the same path", () => {
    state.search = "layout=board";
    renderRail();
    expect(link("/acme/issues?layout=board")).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(link("/acme/issues?layout=table")).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("keeps every destination accessible when collapsed", () => {
    renderRail();
    fireEvent.click(screen.getByRole("button", { name: "collapse" }));
    expect(screen.getByRole("navigation")).toHaveAttribute(
      "data-expanded",
      "false",
    );
    expect(screen.getByRole("link", { name: "Agent chat" })).toHaveAttribute(
      "href",
      "/acme/chat",
    );
    expect(
      document.querySelectorAll("[data-workbench-navigation] a"),
    ).toHaveLength(14);
  });
});
describe("AppRail badges", () => {
  it("counts inbox unread with a static counter", () => {
    state.inboxUnread = 4;
    renderRail();
    const badge = countBadge("/acme/inbox") as
      | (HTMLElement & { animated?: boolean })
      | null;
    expect(badge).toHaveAttribute("aria-label", "4");
    expect(badge?.animated).toBe(false);
  });

  it("shows no attention dot when nothing needs a look", () => {
    renderRail();
    expect(link("/acme/agents")?.querySelector(".bg-warning")).toBeNull();
  });

  it("flags Team when an agent is stuck", () => {
    state.stuck = [{ id: "agent-1" }];
    renderRail();
    expect(link("/acme/agents")?.querySelector(".bg-warning")).not.toBeNull();
  });
});

describe("AppRail chat badge", () => {
  const chatBadge = () => countBadge("/acme/chat");

  it("sums unread messages across sessions", () => {
    state.chatSessions = [
      { id: "a", unread_count: 3 },
      { id: "b", unread_count: 2 },
      { id: "c", unread_count: 0 },
    ];
    renderRail();
    expect(chatBadge()).toHaveAttribute("aria-label", "5");
  });

  it("shows nothing when every session is read", () => {
    state.chatSessions = [{ id: "a", unread_count: 0 }, { id: "b" }];
    renderRail();
    expect(chatBadge()).toBeNull();
  });

  // The thread list zeroes the open session's row; the aggregate must
  // follow, or a reply landing there flashes a count with no matching row.
  it("excludes the session being read on the chat route", () => {
    state.chatSessions = [
      { id: "a", unread_count: 2 },
      { id: "b", unread_count: 3 },
    ];
    state.pathname = "/acme/chat";
    state.chat = { activeSessionId: "a", isOpen: false };
    renderRail();
    expect(chatBadge()).toHaveAttribute("aria-label", "3");
  });

  it("excludes the session open in the floating window off-route", () => {
    state.chatSessions = [
      { id: "a", unread_count: 2 },
      { id: "b", unread_count: 3 },
    ];
    state.chat = { activeSessionId: "a", isOpen: true };
    renderRail();
    expect(chatBadge()).toHaveAttribute("aria-label", "3");
  });

  it("still counts a remembered selection no chat surface is showing", () => {
    state.chatSessions = [
      { id: "a", unread_count: 2 },
      { id: "b", unread_count: 3 },
    ];
    state.chat = { activeSessionId: "a", isOpen: false };
    renderRail();
    expect(chatBadge()).toHaveAttribute("aria-label", "5");
  });

  // Backgrounded, auto mark-read is suppressed (MUL-4485): the reply stays
  // unread and must badge, or the notification is eaten while away.
  it("counts the open session while the app is backgrounded", () => {
    state.chatSessions = [
      { id: "a", unread_count: 2 },
      { id: "b", unread_count: 3 },
    ];
    state.pathname = "/acme/chat";
    state.chat = { activeSessionId: "a", isOpen: true };
    state.appForeground = false;
    renderRail();
    expect(chatBadge()).toHaveAttribute("aria-label", "5");
  });
});
