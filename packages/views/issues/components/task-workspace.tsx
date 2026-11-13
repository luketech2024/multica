"use client";
import { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { Virtuoso, type VirtuosoHandle } from "react-virtuoso";
import type { AgentTask, Issue } from "@multica/core/types";
import { contentReferencesAttachment } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { useAuthStore } from "@multica/core/auth";
import { useCurrentMember } from "@multica/core/permissions";
import { useActorName } from "@multica/core/workspace/hooks";
import { useCommentDraftStore } from "@multica/core/issues/stores";
import { childIssuesOptions, issueDetailOptions, issueTasksOptions, issueTimelineOptions } from "@multica/core/issues/queries";
import { useCancelIssueRun, useRetryIssueRun } from "@multica/core/issues/mutations";
import { currentExecutionIssues, taskWorkspaceProgress, conversationCommand, conversationDestination, canControlWorkspaceExecution, useControlIssueExecution, oneSpecFlowOptions, oneSpecConfigurationOptions } from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@multica/ui/components/ui/sheet";
import { useSidebarSafe } from "@multica/ui/components/ui/sidebar";
import { PanelLeft, X } from "@multica/ui/icons";
import { useT } from "../../i18n";
import { useIssueTimelineActions, useIssueTimelineEvents } from "../hooks/use-issue-timeline";
import { useMarkIssueReadWhileViewing } from "../hooks/use-mark-issue-read-while-viewing";
import { useMarkIssueInboxReadWhileViewing } from "../hooks/use-mark-issue-inbox-read-while-viewing";
import { CurrentIssueRenderContextProvider, type DiscussionReplyTarget } from "../current-issue-render-context";
import { CommentCard, AgentRunComment } from "./comment-card";
import { CommentInput } from "./comment-input";
import { CommentSendReceipt } from "./comment-send-receipt";
import { userInputOptions } from "@multica/core/issues/user-input";
import type { TaskUserInput } from "@multica/core/issues/user-input-schema";
import { UserInputMessages, UserInputComposer } from "./user-input-conversation";
import { toast } from "sonner";
import { conversationRows, type ConversationMessage, type ConversationSource as Source } from "./workspace-conversation";

import { OneSpecFlowPanel, OneSpecRequirementsPanel } from "./onespec-flow-panel";
import { DeliveryReview } from "./delivery-review";
import { DeliveryEvidence } from "./delivery-evidence";
import { ExecutionControl } from "./execution-control";
import { requirementsDiscussion, requirementsReviewReady, requirementsHandoff } from "@multica/core/workbench/configuration";
import { RequirementsSummary } from "./requirements-summary";
const ConversationFooterContext = createContext<ReactNode>(null);
function ConversationFooter() { return useContext(ConversationFooterContext); }
type ConversationTarget = {
    issueId: string;
    reply?: DiscussionReplyTarget;
    explicit?: boolean;
};
type ConversationRow = ConversationMessage & {
    input?: TaskUserInput;
    requirements?: boolean;
    delivery?: boolean;
};
export function TaskWorkspace({ issue, leadingAction, trailingActions, highlightCommentId, highlightRequestToken, readsInbox = true, peek = false, renderDetails }: {
    issue: Issue;
    leadingAction?: ReactNode;
    trailingActions?: ReactNode;
    highlightCommentId?: string;
    highlightRequestToken?: number;
    readsInbox?: boolean;
    peek?: boolean;
    renderDetails: (issueId: string) => ReactNode;
}) {
    const wsId = useWorkspaceId();
    const { t } = useT("issues");
    const children = useQuery(childIssuesOptions(wsId, issue.id));
    const execution = currentExecutionIssues(issue, children.data ?? []);
    const issues = [issue, ...execution];
    const timelines = useQueries({ queries: issues.map((source) => issueTimelineOptions(source.id)) });
    const runs = useQueries({ queries: issues.map((source) => issueTasksOptions(source.id)) });
    const sources = issues.map((source, index) => ({ issue: source, timeline: timelines[index]?.data ?? [], runs: runs[index]?.data ?? [] }));
    const failed = children.isError || timelines.some((query) => query.isError) || runs.some((query) => query.isError);
    const loaded = children.isSuccess && timelines.every((query) => query.isSuccess) && runs.every((query) => query.isSuccess);
    const [initialized, setInitialized] = useState(false);
    useEffect(() => {
        if (loaded)
            setInitialized(true);
    }, [loaded]);
    useMarkIssueReadWhileViewing(wsId, issue.id);
    useMarkIssueInboxReadWhileViewing(wsId, issue.id, readsInbox);
    const retry = () => {
        void children.refetch();
        timelines.forEach((query) => void query.refetch());
        runs.forEach((query) => void query.refetch());
    };
    if (!initialized)
        return <div className="flex min-h-0 flex-1 flex-col p-6">
    <div className="flex items-center gap-3">{leadingAction}<h1 className="flex-1 text-title-sm">{issue.title}</h1>{trailingActions}</div>
    <p role={failed ? "alert" : "status"} className="my-6 text-muted-foreground">{t(($) => failed ? $.next_step.read_failed : $.delivery_review.loading)}</p>
    {failed && <Button onClick={retry}>{t(($) => $.release_readiness.retry)}</Button>}
  </div>;
    return <WorkspaceContent key={issue.id} {...{ issue, sources, leadingAction, trailingActions, highlightCommentId, highlightRequestToken, peek, renderDetails }} dataReady={loaded} onRetry={retry}/>;
}
function WorkspaceContent({ issue, sources, leadingAction, trailingActions, highlightCommentId, highlightRequestToken, peek, renderDetails, dataReady, onRetry }: {
    issue: Issue;
    sources: Source[];
    leadingAction?: ReactNode;
    trailingActions?: ReactNode;
    highlightCommentId?: string;
    highlightRequestToken?: number;
    peek: boolean;
    renderDetails: (id: string) => ReactNode;
    dataReady: boolean;
    onRetry: () => void;
}) {
    const { t } = useT("issues");
    const { t: projectText } = useT("projects");
    const wsId = useWorkspaceId();
    const sidebar = useSidebarSafe();
    const questionQueries = useQueries({ queries: sources.map((source) => userInputOptions(wsId, source.issue.id)) });
    const questions = questionQueries.flatMap((query) => query.data ?? []);
    const [questionId, setQuestionId] = useState<string>();
    const pendingQuestions = questions.filter((question) => question.status === "pending");
    const question = pendingQuestions.find((item) => item.id === questionId) ?? (pendingQuestions.length === 1 ? pendingQuestions[0] : undefined);
    const questionPause = useControlIssueExecution(wsId, issue.id);
    const cancelQuestionRun = useCancelIssueRun(issue.id);
    const member = useCurrentMember(wsId);
    const sidebarInitiallyOpen = useRef(sidebar?.open);
    const setSidebarOpen = sidebar?.setOpen;
    useEffect(() => {
        if (peek || !setSidebarOpen)
            return;
        const initiallyOpen = sidebarInitiallyOpen.current;
        setSidebarOpen(false);
        return () => setSidebarOpen(initiallyOpen ?? true);
    }, [peek, setSidebarOpen]);
    const flow = useQuery(oneSpecFlowOptions(wsId, issue.id));
    const configuration = useQuery(oneSpecConfigurationOptions(wsId, issue.project_id!));
    const execution = sources.slice(1).map((source) => source.issue);
    const progress = taskWorkspaceProgress(issue, execution, sources.flatMap((source) => source.runs));
    const discussing = requirementsDiscussion(issue) && (!flow.data?.flow || flow.data.flow.phase === "requirements");
    const handoff = !!issue.acceptance_criteria?.gate.spec && requirementsHandoff(flow.data?.flow ?? null);
    const flowAttention = !discussing && !handoff && ( (!!flow.data?.flow && ["awaiting_confirmation", "ready"].includes(flow.data.flow.status))
        || (!flow.data?.flow && !!configuration.data?.enabled && !issue.acceptance_criteria?.plan));
    const formal = !discussing && !flowAttention && !flow.data?.flow && ["requirements", "plan", "change", "review", "recovery", "accepted"].includes(progress.step);
    const confirmedAt = issue.acceptance_criteria?.gate.spec?.passed_at;
    const anchorRequirements = !!confirmedAt && !issue.acceptance_criteria?.change && (handoff || (formal && ["requirements", "plan"].includes(progress.step)));
    const deliveryAt = Object.values(issue.acceptance_criteria?.delivery ?? {}).map((evidence) => evidence.recorded_at)
        .sort((left, right) => Date.parse(right) - Date.parse(left))[0];
    const anchorDelivery = formal && ["review", "accepted"].includes(progress.step) && !!deliveryAt;
    const rows = useMemo(() => {
        const entries: ConversationRow[] = conversationRows(sources);
        for (const input of questions) {
            const source = sources.find((item) => item.issue.id === input.issue_id);
            if (source) entries.push({ input, source: source.issue, id: input.id, time: input.created_at, runs: [], replies: [] });
        }
        if (anchorRequirements && confirmedAt) {
            // Run timestamps are serialized to seconds. Confirmation precedes
            // the execution it enqueues within that same second.
            const time = new Date(Math.floor(Date.parse(confirmedAt) / 1000) * 1000).toISOString();
            entries.push({ requirements: true, source: issue, id: `requirements:${issue.acceptance_criteria!.version}`, time, runs: [], replies: [] });
        }
        if (anchorDelivery && deliveryAt) {
            entries.push({ delivery: true, source: issue, id: `delivery:${issue.acceptance_criteria!.version}`, time: deliveryAt, runs: [], replies: [] });
        }
        return entries.sort((left, right) => Date.parse(left.time) - Date.parse(right.time)
            || Number(!!right.requirements) - Number(!!left.requirements) || left.id.localeCompare(right.id));
    }, [sources, questions, anchorRequirements, confirmedAt, anchorDelivery, deliveryAt, issue]);
    const [target, setTarget] = useState<ConversationTarget>(() => {
        const drafts = useCommentDraftStore.getState().drafts;
        const existing = Object.entries(drafts).filter(([key, draft]) => sources.some((source) => key === `new:${source.issue.id}` || (key.startsWith(`reply:${source.issue.id}:`) && !key.startsWith(`reply:${source.issue.id}:input:`)))
            && (draft.content.trim() || draft.attachments.length || draft.annotations?.length))
            .sort((left, right) => right[1].updatedAt - left[1].updatedAt)[0];
        if (existing) {
            const [kind, issueId, threadId] = existing[0].split(":");
            if (issueId)
                return { issueId, reply: kind === "reply" && threadId ? {
                        parentId: existing[1].replyTarget?.commentId ?? threadId,
                        draftThreadId: threadId, name: existing[1].replyTarget?.actorName ?? issue.title,
                    } : undefined };
        }
        const executing = ["planned", "running", "blocked"].includes(progress.step);
        return { issueId: highlightCommentId ? issue.id : executing && execution.length === 1 ? execution[0]!.id : issue.id };
    });
    const [detailsId, setDetailsId] = useState(issue.id);
    const [detailsOpen, setDetailsOpen] = useState(false);
    const [history, setHistory] = useState(!!highlightCommentId);
    const virtuoso = useRef<VirtuosoHandle>(null);
    const composerRef = useRef<HTMLDivElement>(null);
    const cutoff = sources.slice(1).flatMap((source) => source.runs.map((run) => run.created_at)).sort()[0]
        ?? execution.map((child) => child.created_at).sort()[0];
    const recentRows = cutoff ? rows.filter((row) => row.requirements || Date.parse(row.time) >= Date.parse(cutoff)).slice(-20) : rows.slice(-20);
    const rowsShown = history ? rows : recentRows;
    const landedHighlight = useRef("");
    useEffect(() => {
        if (!highlightCommentId)
            return;
        const key = `${highlightCommentId}:${highlightRequestToken ?? 0}`;
        if (landedHighlight.current === key)
            return;
        setHistory(true);
        const index = rows.findIndex((row) => row.id === highlightCommentId || row.replies.some((reply) => reply.id === highlightCommentId));
        if (index >= 0 && history) {
            virtuoso.current?.scrollToIndex({ index, align: "center" });
            landedHighlight.current = key;
        }
    }, [highlightCommentId, highlightRequestToken, rows, history]);
    const chooseReply = (source: Issue, reply: DiscussionReplyTarget) => {
        setTarget({ issueId: source.id, reply });
        requestAnimationFrame(() => {
            const shell = composerRef.current?.querySelector<HTMLButtonElement>("[data-testid=comment-composer-shell]");
            if (shell)
                shell.click();
            else
                composerRef.current?.querySelector<HTMLElement>("[contenteditable=true]")?.focus();
        });
    };
    const handoffStageLabels: Record<string, string> = {
        requirements: projectText(($) => $.onespec.requirements), design: projectText(($) => $.onespec.design),
        plan: projectText(($) => $.onespec.plan), branch: projectText(($) => $.onespec.branch),
        release_preparation: projectText(($) => $.onespec.release_preparation),
    };
    const latestDiscussionRun = sources[0]?.runs.toSorted((a, b) => b.created_at.localeCompare(a.created_at))[0];
    const emptyDiscussion = discussing && !issue.acceptance_criteria?.items.length && !issue.description?.trim() && !latestDiscussionRun && execution.length === 0;
    useEffect(() => {
        if (emptyDiscussion && rows.length === 0) composerRef.current?.querySelector<HTMLButtonElement>("[data-testid=comment-composer-shell]")?.click();
    }, [emptyDiscussion, rows.length]);
    const actionable = progress.attention.filter((child) => !child.assignee_id || sources.find((source) => source.issue.id === child.id)?.runs.toSorted((a, b) => b.created_at.localeCompare(a.created_at))[0]?.status === "failed");
    const needsAction = !issue.workbench_pause && (formal || flowAttention || actionable.length > 0 || progress.startable.length > 0);
    const status = discussing ? (progress.active.length || progress.queued.length ? t(($) => $.discussion.replying)
        : latestDiscussionRun?.status === "cancelled" ? t(($) => $.discussion.paused)
        : requirementsReviewReady(issue.acceptance_criteria, flow.data?.flow) ? t(($) => $.discussion.ready)
        : rows.length ? t(($) => $.discussion.waiting) : t(($) => $.discussion.title)) : issue.workbench_pause ? t(($) => $.execution_control[issue.workbench_pause!.phase])
        : progress.paused.length ? progress.paused.map((child) => `${child.identifier} · ${t(($) => $.execution_control[child.workbench_pause!.phase])}`).join(" · ")
            : pendingQuestions.length ? t(($) => $.conversation.waiting)
            : handoff ? handoffStageLabels[flow.data!.flow!.phase] ?? t(($) => $.task_workspace.attention)
            : formal ? t(($) => $.next_step.stages[progress.step])
                : progress.attention.length ? t(($) => $.task_workspace.blocked, { count: progress.attention.length })
                    : needsAction ? t(($) => $.task_workspace.attention)
                        : progress.active.length ? t(($) => $.task_workspace.running, { count: progress.active.length })
                            : progress.queued.length ? t(($) => $.task_workspace.queued, { count: progress.queued.length })
                                : t(($) => $.next_step.stages[progress.step]);
    const openDetails = (id: string) => { setDetailsId(id); setDetailsOpen(true); };
    const requirementsSummary = (<RequirementsSummary issue={issue} onDiscuss={() => {
            setTarget({ issueId: issue.id, explicit: true });
            requestAnimationFrame(() => {
              composerRef.current?.querySelector<HTMLButtonElement>("[data-testid=comment-composer-shell]")?.click();
              composerRef.current?.querySelector<HTMLElement>("[contenteditable=true]")?.focus();
            });
          }}/>);
    const actionMessage = (<div className="pb-4">
          {questionQueries.some((query) => query.isError) && <p role="alert">{t(($) => $.conversation.load_failed)} <Button size="sm" variant="ghost" onClick={() => questionQueries.forEach((query) => void query.refetch())}>{t(($) => $.release_readiness.retry)}</Button></p>}
          {(!dataReady || flow.isError || configuration.isError) && <p role="alert">{t(($) => $.next_step.read_failed)} <Button variant="ghost" onClick={() => { onRetry(); void flow.refetch(); void configuration.refetch(); }}>{t(($) => $.release_readiness.retry)}</Button></p>}
          {(issue.workbench_pause || progress.paused.length > 0) && <p role="status" className="py-3 text-caption text-muted-foreground">{status}</p>}
          {issue.workbench_pause?.phase === "unknown" && <ExecutionControl issue={issue}/>}
          {!anchorRequirements && (discussing || handoff) && requirementsSummary}
          {discussing && latestDiscussionRun?.status === "failed" && <ExecutionAction issue={issue} latest={latestDiscussionRun} startable={false} onDiscuss={() => setTarget({ issueId: issue.id })} onDetails={() => openDetails(issue.id)}/>}
          {needsAction && ((!anchorRequirements && !anchorDelivery) || actionable.length > 0 || progress.startable.length > 0) && <section data-testid="workspace-action" aria-label={t(($) => $.task_workspace.system_message)} className="rounded-xl bg-muted/40 p-4">
            <fieldset disabled={!dataReady} className="space-y-4">
              {flowAttention ? <OneSpecFlowPanel issue={issue} compact/> : formal && !anchorRequirements && !anchorDelivery && (["review", "recovery", "accepted"].includes(progress.step)
                ? <><DeliveryEvidence issue={issue}/><DeliveryReview issue={issue}/></>
                : <OneSpecRequirementsPanel issue={issue}/>)}
              {[...actionable, ...progress.startable].map((child) => <ExecutionAction key={child.id} issue={child} startable={progress.startable.includes(child)} latest={sources.find((source) => source.issue.id === child.id)?.runs.toSorted((a, b) => b.created_at.localeCompare(a.created_at))[0]} onDiscuss={() => setTarget({ issueId: child.id, explicit: true })} onDetails={() => openDetails(child.id)}/>)}
            </fieldset>
          </section>}
        </div>);
    return <div data-testid="task-workspace" className="flex min-h-0 min-w-0 flex-1 flex-col bg-background">
    {sources.map((source) => <TimelineEvents key={source.issue.id} issueId={source.issue.id}/>)}
    <header className="flex shrink-0 items-center gap-3 border-b px-6 py-4 max-md:px-3">
      {leadingAction ?? (sidebar && <Button variant="ghost" size="icon-sm" aria-label={t(($) => $.task_workspace.navigation)} onClick={sidebar.toggleSidebar}><PanelLeft /></Button>)}
      <div className="min-w-0 flex-1">
        <p className="text-caption text-muted-foreground">{issue.identifier}</p>
        <h1 className="break-words text-title-sm font-medium">{issue.title}</h1>
        <p role="status" data-testid="workspace-status" className="text-caption text-muted-foreground">{status}</p>
      </div>
      <Sheet open={detailsOpen} onOpenChange={setDetailsOpen}>
        <SheetTrigger render={<Button variant="ghost" size="sm" onClick={() => setDetailsId(issue.id)}/>}>{t(($) => $.task_workspace.details)}</SheetTrigger>
        <SheetContent className="w-full sm:max-w-3xl" side="right">
          <SheetHeader><SheetTitle>{t(($) => $.task_workspace.details)}</SheetTitle></SheetHeader>
          <select aria-label={t(($) => $.task_workspace.details)} className="mx-4 rounded border bg-background p-2" value={detailsId} onChange={(event) => setDetailsId(event.target.value)}>
            {sources.map((source) => <option key={source.issue.id} value={source.issue.id}>{source.issue.identifier} · {source.issue.title}</option>)}
          </select>
          <Button variant="ghost" onClick={() => { setTarget({ issueId: detailsId, explicit: true }); setDetailsOpen(false); }}>{t(($) => $.task_workspace.discuss)}</Button>
          <div className="flex min-h-0 flex-1 flex-col">{renderDetails(detailsId)}</div>
        </SheetContent>
      </Sheet>
      {trailingActions}
    </header>
    <div className="mx-auto flex min-h-0 w-full max-w-[880px] flex-1 flex-col px-6 max-md:px-3">
    <div className="py-4 text-center">
          {!history && rows.length > rowsShown.length && <Button variant="ghost" size="sm" onClick={() => setHistory(true)}>{t(($) => $.task_workspace.history, { count: rows.length - rowsShown.length })}</Button>}
          {!rows.length && <p className="text-caption text-muted-foreground">{t(($) => emptyDiscussion ? $.discussion.invitation : $.task_workspace.empty)}</p>}
        </div>
      <ConversationFooterContext.Provider value={actionMessage}>      <Virtuoso components={{ Footer: ConversationFooter }} ref={virtuoso} data={rowsShown} followOutput="smooth" increaseViewportBy={500} computeItemKey={(_, row) => `${row.source.id}:${row.id}`} itemContent={(_, row) => row.delivery ? <section data-testid="workspace-delivery" className="my-4 space-y-4 rounded-xl bg-muted/40 p-4"><fieldset disabled={!dataReady} className="space-y-4"><DeliveryEvidence issue={issue}/><DeliveryReview issue={issue}/></fieldset></section> : row.requirements ? <section data-testid="workspace-confirmed-requirements" className="my-4 rounded-xl bg-muted/40 p-4"><fieldset disabled={!dataReady}>{handoff ? requirementsSummary : <OneSpecRequirementsPanel issue={issue}/>}</fieldset></section> : row.input ? <UserInputMessages requests={[row.input]} selectedId={question?.id} onSelect={setQuestionId} onCorrect={(request, text) => { setTarget({ issueId: request.issue_id, explicit: true }); useCommentDraftStore.getState().appendToDraftContent(`new:${request.issue_id}`, text); }}/> : <WorkspaceMessage row={row} onReply={chooseReply} highlightCommentId={highlightCommentId} onDetails={() => openDetails(row.source.id)}/>}/></ConversationFooterContext.Provider>    </div>
    <div ref={composerRef} className="mx-auto w-full max-w-[880px] shrink-0 px-6 pb-4 pt-2 max-md:px-3">
      {question ? <UserInputComposer key={question.id} wsId={wsId} request={question} pauseControl={canControlWorkspaceExecution(issue, member.role) ? { pending: questionPause.isPending || cancelQuestionRun.isPending, onPause: async () => {
        try { if (discussing) { for (const run of sources[0]?.runs ?? []) { if (["running", "queued", "dispatched", "waiting_local_directory"].includes(run.status)) await cancelQuestionRun.mutateAsync(run.id); } } else await questionPause.mutateAsync({ action: "pause" }); return true; }
        catch { toast.error(t(($) => $.execution_control.failed)); return false; }
      } } : undefined} />
        : <WorkspaceComposer key={`${target.issueId}:${target.reply?.parentId ?? "task"}`} parent={issue} discussing={discussing} sources={sources} target={target} dataReady={dataReady} onClearReply={() => setTarget({ issueId: target.issueId })}/>}
    </div>
  </div>;
}
function WorkspaceMessage({ row, onReply, highlightCommentId, onDetails }: {
    row: ConversationRow;
    onReply: (issue: Issue, reply: DiscussionReplyTarget) => void;
    highlightCommentId?: string;
    onDetails: () => void;
}) {
    const { t } = useT("issues");
    const user = useAuthStore((state) => state.user);
    const wsId = useWorkspaceId();
    const member = useCurrentMember(wsId);
    const actions = useIssueTimelineActions(row.source.id, user?.id);
    if (row.entry?.type === "activity") {
        const pause = row.entry.details?.pause;
        const phase = typeof pause === "object" && pause && "phase" in pause
            ? (["stopping", "unknown", "paused", "resuming"] as const).find((value) => value === pause.phase) : undefined;
        return <p className="py-3 text-caption text-muted-foreground">{row.source.identifier} · {t(($) => phase ? $.execution_control[phase] : $.execution_control.updated)}</p>;
    }
    const commandText = row.entry?.content?.replace(/^\/note\s+/i, "");
    if (row.entry?.content?.startsWith("/note ") && commandText && conversationCommand(commandText)) {
        const own = row.entry.actor_type === "member" && row.entry.actor_id === user?.id;
        return <div className={`my-3 w-[88%] rounded-xl p-4 ${own ? "ml-auto bg-accent" : "bg-card"}`} data-message-side={own ? "right" : "left"}><p>{commandText}</p></div>;
    }
    const props = row.entry ? {
        issueId: row.source.id,
        entry: actions.timeline.find((entry) => entry.id === row.entry!.id) ?? row.entry,
        replies: row.replies.map((reply) => actions.timeline.find((entry) => entry.id === reply.id) ?? reply),
        runs: row.runs,
        currentUserId: user?.id, canModerate: ["owner", "admin"].includes(member.role ?? ""),
        onReply: actions.submitReply, onEdit: actions.editComment, onDelete: actions.deleteComment,
        onToggleReaction: actions.toggleReaction, onResolveToggle: actions.toggleResolveComment,
        highlightedCommentId: highlightCommentId,
    } : undefined;
    return <CurrentIssueRenderContextProvider value={{ id: row.source.id, identifier: row.source.identifier, status: row.source.status, compact: true, composeReply: (reply) => onReply(row.source, {
                ...reply,
                draftThreadId: reply.draftThreadId ?? row.entry?.id,
                steerByDefault: reply.steerByDefault ?? ((task) => row.runs.some((run) => run.task.id === task.id) || row.run?.task.id === task.id),
            }) }}>
    <div className="pb-5" data-source-issue={row.source.id} data-conversation-row={row.id}>
      <Button variant="ghost" size="sm" className="mb-1 max-w-full truncate text-caption text-muted-foreground" onClick={onDetails}>{row.source.identifier} · {row.source.title}</Button>
      {row.run ? <AgentRunComment run={row.run} standalone commentProps={props} highlightedId={highlightCommentId}/> : props && <CommentCard {...props}/>}
    </div>
  </CurrentIssueRenderContextProvider>;
}
function WorkspaceComposer({ parent, sources, target, onClearReply, dataReady, discussing }: {
    parent: Issue;
    discussing: boolean;
    sources: Source[];
    target: ConversationTarget;
    onClearReply: () => void;
    dataReady: boolean;
}) {
    const { t } = useT("issues");
    const wsId = useWorkspaceId();
    const user = useAuthStore((state) => state.user);
    const client = useQueryClient();
    const member = useCurrentMember(wsId);
    const { getActorName } = useActorName();
    const draftKey = target.reply ? `reply:${target.issueId}:${target.reply.draftThreadId ?? target.reply.parentId}` as const : `new:${target.issueId}` as const;
    const savedDraft = useCommentDraftStore((state) => state.drafts[draftKey]);
    const [draft, setDraft] = useState(() => savedDraft?.content ?? "");
    const [choice, setChoice] = useState<string | undefined>(() => savedDraft?.destinationIssueId);
    const execution = sources.slice(1).map((source) => source.issue);
    const inferred = conversationDestination(parent, execution, draft);
    const destination = target.reply ? target.issueId : inferred === parent.id ? parent.id : target.explicit ? target.issueId : choice ?? inferred;
    const issueId = destination ?? parent.id;
    const actions = useIssueTimelineActions(issueId, user?.id);
    const [error, setError] = useState<string>();
    const source = sources.find((item) => item.issue.id === issueId);
    const hasDraftUploads = savedDraft?.attachments.some((upload) => upload.status === "uploading" || (upload.status === "uploaded" && contentReferencesAttachment(draft, upload.attachment)));
    const attachmentMismatch = !!hasDraftUploads && !!savedDraft?.destinationIssueId && savedDraft.destinationIssueId !== issueId;
    useEffect(() => {
        if (savedDraft && draft.trim() && destination && !hasDraftUploads)
            useCommentDraftStore.getState().setDestination(draftKey, destination);
    }, [draft, destination, draftKey, savedDraft, hasDraftUploads]);
    const paused = sources.filter((item) => item.issue.workbench_pause);
    const resumeIssue = parent.workbench_pause ? parent : paused.length === 1 ? paused[0]!.issue : source?.issue;
    const pauseControl = useControlIssueExecution(wsId, parent.id);
    const resumeControl = useControlIssueExecution(wsId, resumeIssue?.id ?? parent.id);
    const cancelReply = useCancelIssueRun(parent.id);
    const command = conversationCommand(draft);
    const localCommand = command === "pause" || (command === "resume" && !!resumeIssue?.workbench_pause);
    const canControl = canControlWorkspaceExecution(parent, member.role);
    const active = sources.some((item) => item.runs.some((run) => ["running", "queued", "dispatched", "waiting_local_directory"].includes(run.status)));
    const stopping = parent.workbench_pause?.phase === "stopping";
    const busy = pauseControl.isPending || resumeControl.isPending || cancelReply.isPending;
    const reconcile = pauseControl.mutateAsync;
    useEffect(() => {
        if (!stopping || !canControl || busy || !parent.revision)
            return;
        const timer = setTimeout(() => {
            void reconcile({ action: "reconcile" }).catch((cause: unknown) => {
                setError(cause instanceof Error ? cause.message : t(($) => $.next_step.read_failed));
            });
        }, 5000);
        return () => clearTimeout(timer);
    }, [stopping, canControl, busy, parent.revision, reconcile, t]);
    const recordedCommand = useRef<{
        content: string;
        id: string;
    } | undefined>(undefined);
    const control = async (action: "pause" | "resume") => {
        if (!canControl)
            throw new Error(t(($) => $.task_workspace.control_unavailable));
        if (discussing && action === "pause") {
            for (const run of sources[0]?.runs ?? []) {
                if (["running", "queued", "dispatched", "waiting_local_directory"].includes(run.status)) await cancelReply.mutateAsync(run.id);
            }
            return;
        }
        const controlIssue = action === "pause" ? parent : resumeIssue;
        if (!controlIssue)
            throw new Error(t(($) => $.task_workspace.choose_target));
        if (action === "pause" && controlIssue.workbench_pause)
            return;
        if (action === "resume" && controlIssue.workbench_pause?.phase !== "paused")
            throw new Error(t(($) => $.task_workspace.wait_for_stop));
        await (action === "pause" ? pauseControl : resumeControl).mutateAsync({ action });
        await Promise.all(sources.flatMap((item) => [
            client.invalidateQueries({ queryKey: issueTasksOptions(item.issue.id).queryKey }),
            client.invalidateQueries({ queryKey: issueTimelineOptions(item.issue.id).queryKey }),
        ]));
    };
    const requestPause = async () => {
        setError(undefined);
        try {
            await control("pause");
        }
        catch (cause) {
            setError(cause instanceof Error ? cause.message : t(($) => $.next_step.read_failed));
        }
    };
    const replyEntry = target.reply ? source?.timeline.find((entry) => entry.id === target.reply!.parentId) : undefined;
    const unavailable = !dataReady || !source || (!!target.reply && (!replyEntry || !!replyEntry.deleted_at));
    const send = async (content: string, attachments?: string[], suppress?: string[], steer?: string[]) => {
        setError(undefined);
        try {
            const operation = conversationCommand(content);
            if (operation === "pause" || (operation === "resume" && resumeIssue?.workbench_pause)) {
                if (attachments?.length)
                    throw new Error(t(($) => $.task_workspace.control_text_only));
                if (!canControl)
                    throw new Error(t(($) => $.task_workspace.control_unavailable));
                if (operation === "pause" && parent.workbench_pause)
                    return true;
                if (recordedCommand.current?.content !== content) {
                    const id = await actions.submitComment(`/note ${content}`);
                    if (!id)
                        return false;
                    recordedCommand.current = { content, id };
                }
                await control(operation);
                const id = recordedCommand.current.id;
                recordedCommand.current = undefined;
                return id;
            }
            const inferredNow = conversationDestination(parent, execution, content);
            const resolved = target.reply ? target.issueId : inferredNow === parent.id ? parent.id : target.explicit ? target.issueId : choice ?? inferredNow;
            if (!resolved || resolved !== issueId)
                throw new Error(t(($) => $.task_workspace.choose_target));
            if (attachmentMismatch)
                throw new Error(t(($) => $.task_workspace.attachment_destination));
            if (issueId !== parent.id) {
                const [current, children] = await Promise.all([
                    client.fetchQuery({ ...issueDetailOptions(wsId, parent.id), staleTime: 0 }),
                    client.fetchQuery({ ...childIssuesOptions(wsId, parent.id), staleTime: 0 }),
                ]);
                if (!current || !currentExecutionIssues(current, children).some((child) => child.id === issueId))
                    throw new Error(t(($) => $.task_workspace.unavailable));
            }
            if (target.reply) {
                const current = await client.fetchQuery({ ...issueTimelineOptions(issueId), staleTime: 0 });
                const reply = current.find((entry) => entry.id === target.reply!.parentId);
                if (!reply || reply.deleted_at)
                    throw new Error(t(($) => $.task_workspace.unavailable));
                return await actions.submitReply(target.reply.parentId, source?.issue.workbench_pause || parent.workbench_pause ? `/note ${content}` : content, attachments, suppress, steer);
            }
            return await actions.submitComment(source?.issue.workbench_pause || parent.workbench_pause ? `/note ${content}` : content, attachments, suppress, steer);
        }
        catch (cause) {
            setError(cause instanceof Error ? cause.message : t(($) => $.next_step.read_failed));
            return false;
        }
    };
    return <div>
    {!target.reply && destination && <p className="mb-2 text-caption text-muted-foreground" data-testid="conversation-recipient">{source?.issue.identifier} · {source?.issue.assignee_id && source.issue.assignee_type ? getActorName(source.issue.assignee_type, source.issue.assignee_id) : t(($) => $.task_workspace.coordinator)}</p>}
    {!destination && draft.trim() && !localCommand && <div role="status" className="mb-3 space-y-2"><p className="text-caption">{t(($) => $.task_workspace.choose_target)}</p><div className="flex flex-wrap gap-2">{sources.map((item) => <Button key={item.issue.id} variant="outline" size="sm" onClick={() => setChoice(item.issue.id)}>{item.issue.identifier} · {item.issue.title}</Button>)}</div></div>}
    {target.reply && <div className="mb-2 flex items-center gap-2 text-caption"><span className="min-w-0 flex-1 truncate">{t(($) => $.composer_ui.replying_to, { name: target.reply.name })} · {replyEntry?.content}</span><Button size="icon-xs" variant="ghost" aria-label={t(($) => $.composer_ui.back_to_task)} onClick={onClearReply}><X /></Button></div>}
    {target.reply?.correction && <p className="mb-2 text-caption text-muted-foreground">{t(($) => $.conversation.correction_hint)}</p>}
    {(unavailable || error || attachmentMismatch) && <p role="alert" className="text-caption text-destructive">{error ?? t(($) => attachmentMismatch ? $.task_workspace.attachment_destination : $.task_workspace.unavailable)}</p>}
    <CommentSendReceipt comment={actions.lastSubmittedComment}/>
    <CommentInput issueId={target.issueId} routingIssueId={issueId} onDraftChange={setDraft} localCommand={localCommand || !!source?.issue.workbench_pause || !!parent.workbench_pause} compact pauseControl={canControl && ((active && !parent.workbench_pause) || stopping || pauseControl.isPending) ? { pending: busy || stopping, onPause: () => void requestPause() } : undefined} parentId={target.reply?.parentId} draftThreadId={target.reply?.draftThreadId} steerByDefault={target.reply?.steerByDefault ?? (() => true)} onEditAnnotation={target.reply?.onEditAnnotation} docked sendDisabled={unavailable || busy || attachmentMismatch || (!destination && !localCommand)} onSubmit={send}/>
  </div>;
}
function ExecutionAction({ issue, latest, startable, onDiscuss, onDetails }: {
    issue: Issue;
    latest?: AgentTask;
    startable: boolean;
    onDiscuss: () => void;
    onDetails: () => void;
}) {
    const { t } = useT("issues");
    const start = useRetryIssueRun(issue.id);
    return <div className="space-y-2 border-t pt-3">
    <p className="text-label">{issue.title}</p>
    {latest?.error && <p className="break-words text-caption text-destructive">{latest.error}</p>}
    <div className="flex flex-wrap gap-2">
      {(startable || latest?.status === "failed") && <Button disabled={start.isPending || start.isSuccess} onClick={() => start.mutate(latest?.id)}>{t(($) => start.isPending || start.isSuccess ? $.next_step.starting : latest?.status === "failed" ? $.release_readiness.retry : $.next_step.start_execution)}</Button>}
      <Button variant="outline" onClick={onDiscuss}>{t(($) => $.task_workspace.discuss)}</Button>
      <Button variant="ghost" onClick={onDetails}>{t(($) => $.task_workspace.details)}</Button>
    </div>
    {issue.workbench_pause && <ExecutionControl issue={issue}/>}
    {start.isError && <p role="alert" className="text-caption text-destructive">{start.error.message}</p>}
  </div>;
}
function TimelineEvents({ issueId }: {
    issueId: string;
}) {
    useIssueTimelineEvents(issueId);
    return null;
}
