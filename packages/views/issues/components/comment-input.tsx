"use client";

import { useRef, useState, useEffect, useMemo, useCallback } from "react";
import { cn } from "@multica/ui/lib/utils";
import { ContentEditor, type ContentEditorRef, useFileDropZone, FileDropOverlay, useLazyEditor, useUploadGate, useComposerSubmit } from "../../editor";
import { FileUploadButton } from "@multica/ui/components/common/file-upload-button";
import { Button } from "@multica/ui/components/ui/button";
import { ArrowUp, Loader2, Square } from "@multica/ui/icons";
import { contentReferencesAttachment, type AgentTask } from "@multica/core/types";
import { useShortcut } from "@multica/core/shortcuts";
import { ShortcutKeycaps } from "../../common/shortcut-keycaps";
import { getCurrentSlug } from "@multica/core/platform";
import { useCommentDraftStore } from "@multica/core/issues/stores";
import { composeAnnotatedReply, hasReplyIntent } from "@multica/core/drafts/reply-annotation";
import { ReplyAnnotations } from "./reply-annotations";
import { useT } from "../../i18n";
import { CommentTriggerChips } from "./comment-trigger-chips";
import { commentTriggerPreviewSignature, useCommentTriggerPreview } from "../hooks/use-comment-trigger-preview";
import { useRecipientActions } from "../hooks/use-recipient-actions";
import { SteerAttachmentNotice } from "./steer-attachment-notice";
import { useStopRunsBeforeSend } from "./use-stop-runs-before-send";
import { useCommentUploads } from "./use-comment-uploads";
import { useQuickActionMenu } from "../hooks/use-quick-action-menu";
import { useStickyComposer } from "../hooks/use-sticky-composer";

interface CommentInputProps {
  issueId: string;
  parentId?: string;
  draftThreadId?: string;
  steerByDefault?: (task: AgentTask) => boolean;
  docked?: boolean;
  sendDisabled?: boolean;
  routingIssueId?: string;
  onDraftChange?: (content: string) => void;
  localCommand?: boolean;
  compact?: boolean;
  textOnly?: boolean;
  inputLabel?: string;
  submitLabel?: string;
  pauseControl?: { pending: boolean; onPause: () => void };
  /** Resolves true on success, false on failure. The composer keeps the text
   *  (editor locked + button spinning) until this settles, then clears only on
   *  success — a failed send must not silently discard the user's draft. */
  onSubmit: (content: string, attachmentIds?: string[], suppressAgentIds?: string[], steerTaskIds?: string[]) => Promise<string | boolean>;
  /** Called after the server accepts the comment and the composer is cleared. */
  onAccepted?: (commentId: string) => void;
  onEditAnnotation?: (id: string) => boolean;
}

// A top-level comment belongs to no thread, so it takes a running turn by
// default only when that turn has no thread of its own (an assignment run).
function steerTopLevelByDefault(task: AgentTask): boolean {
  return !task.trigger_comment_id;
}

function CommentInput({ issueId, parentId, draftThreadId, steerByDefault: threadSteerByDefault, onSubmit, onAccepted, onEditAnnotation, docked, sendDisabled, routingIssueId = issueId, onDraftChange, localCommand, compact, textOnly, inputLabel, submitLabel, pauseControl }: CommentInputProps) {
  const { t } = useT("issues");
  const { t: tEditor } = useT("editor");
  const placeholder = inputLabel ?? (parentId ? t(($) => $.reply.placeholder) : t(($) => $.comment.leave_comment_placeholder));
  const sendShortcut = useShortcut("send");
  const editorRef = useRef<ContentEditorRef>(null);
  // Sending mid-upload would strip the pending image's blob URL out of the
  // markdown and bind no attachment id — the comment posts without the file.
  const uploadGate = useUploadGate(editorRef);
  // Subscribe to the persisted draft so text written into it from outside
  // this composer shows up even while it is mounted.
  // Quick actions in the `/` menu: picking one inserts the cached
  // body so the user can edit before sending, instead of firing immediately.
  const quickActionMenu = useQuickActionMenu();
  const draftKey = parentId ? `reply:${issueId}:${draftThreadId ?? parentId}` as const : `new:${issueId}` as const;
  const [initialDraft] = useState(() =>
    useCommentDraftStore.getState().getDraft(draftKey),
  );
  const persistedDraft = useCommentDraftStore((store) => store.getDraft(draftKey));
  const [content, setContent] = useState(initialDraft ?? "");
  useEffect(() => { onDraftChange?.(content); }, [content, onDraftChange]);
  const [expanded, setExpanded] = useState(!!initialDraft?.trim());
  const [draftState, setDraftState] = useState<"restored" | "saved" | "failed" | "clear_failed" | null>(initialDraft?.trim() ? "restored" : null);
  const [sendFailed, setSendFailed] = useState(false);
  const shellRef = useRef<HTMLButtonElement>(null);
  const [isEmpty, setIsEmpty] = useState(() => !initialDraft?.trim());
  const annotations = useCommentDraftStore((s) => s.getAnnotations(draftKey));
  const composedContent = useMemo(() => composeAnnotatedReply(content, annotations), [content, annotations]);
  const canSend = annotations.length ? hasReplyIntent(content, annotations) : !isEmpty;
  const triggerPreview = useCommentTriggerPreview({ issueId: routingIssueId, parentId, content: canSend && !localCommand ? composedContent : "" });
  const steerByDefault = useCallback((task: AgentTask) => parentId
    ? task.trigger_comment_id === parentId
    : steerTopLevelByDefault(task), [parentId]);
  // Uploads for this composer session (MUL-5181). Owned by the module-level
  // coordinator and persisted in the draft store, so closing/scrolling the
  // composer away no longer drops an in-flight upload — its result lands in the
  // draft. `attachments` (completed rows) drives both the submit `attachment_ids`
  // payload and the editor's AttachmentDownloadProvider; `uploads` drives the
  // status chips (uploading / failed / interrupted).
  // `gate` widens the editor gate with coordinator-owned placeholders, so a
  // composer reopened over a still-in-flight upload cannot send past it.
  const { attachments: pendingAttachments, handleUpload, gate } =
    useCommentUploads(draftKey, { issueId: routingIssueId }, uploadGate, editorRef);
  const uploadToDestination = (file: File, uploadId?: string) => {
    if (sendDisabled) return Promise.reject(new Error(t(($) => $.task_workspace.choose_target)));
    const store = useCommentDraftStore.getState();
    const saved = store.drafts[draftKey];
    if (saved?.attachments.some((upload) => upload.status === "uploading" || (upload.status === "uploaded" && contentReferencesAttachment(content, upload.attachment))) && saved.destinationIssueId && saved.destinationIssueId !== routingIssueId)
      return Promise.reject(new Error(t(($) => $.task_workspace.attachment_destination)));
    store.setDestination(draftKey, routingIssueId);
    return handleUpload(file, uploadId);
  };
  const hasAttachments = useMemo(
    () => pendingAttachments.some((a) => contentReferencesAttachment(content, a)) || gate.uploading,
    [pendingAttachments, content, gate.uploading],
  );
  const { recipients, attachmentsBlockSteer, routing, setAction, reset: resetRecipients, ready: recipientsReady, isError: recipientsError, retry: retryRecipients } = useRecipientActions({
    issueId: routingIssueId,
    agents: triggerPreview.agents,
    allowSteer: true,
    hasAttachments,
    steerByDefault: threadSteerByDefault ?? steerByDefault,
    resetKey: `${draftKey}:${routingIssueId}`,
  });
  const previewReady = triggerPreview.isCurrent && recipientsReady;
  const previewError = triggerPreview.isError || recipientsError;
  const stopRunsBeforeSend = useStopRunsBeforeSend(routingIssueId);

  // Readonly-first: the composer renders as a same-looking static shell until
  // the user shows intent (click / keyboard / file drop). An unsent draft is
  // standing intent — mount the real editor immediately so the draft is
  // visible and editable, exactly like the pre-lazy behavior.
  const lazy = useLazyEditor({
    initialActive:
      !!initialDraft?.trim() ||
      useCommentDraftStore.getState().getUploads(draftKey).length > 0,
    editorRef,
  });
  const { isDragOver, dropZoneProps } = useFileDropZone({
    onDrop: (files) => { if (!textOnly) { setExpanded(true); lazy.uploadOrQueue(files); } },
  });
  // Sticky preference (Settings → Preferences): issue-detail pins this
  // composer to the bottom of the scroll viewport when enabled. Shared with
  // the host so the height cap below can never outlive the pinning.
  const preferredSticky = useStickyComposer();
  const sticky = docked ?? preferredSticky;

  // Draft persistence. Hydrate from store on mount via `defaultValue` above
  // (ContentEditorRef has no setContent, so this is the only injection point).
  // Flush on every onUpdate (debounced upstream) + visibilitychange/pagehide
  // so tab close / mobile background doesn't lose work. Cleared on submit.
  const writeDraft = useCommentDraftStore((s) => s.setDraft);
  const setDraft = useCallback((key: typeof draftKey, value: string) => {
    try {
      writeDraft(key, value);
      setDraftState(value.trim() && getCurrentSlug() ? "saved" : null);
    } catch {
      setDraftState("failed");
    }
  }, [writeDraft]);
  useEffect(() => {
    // Only an actual stored value is an external update. An absent entry is
    // not a command to erase the editor: it is normal during the synchronous
    // keystroke -> store handoff and after explicit submit cleanup.
    if (persistedDraft === undefined) return;
    const next = persistedDraft;
    if (next === content) return;
    setContent(next);
    setIsEmpty(!next.trim());
    if (next.trim()) { lazy.activate(); setExpanded(true); }
  }, [content, lazy, persistedDraft]);
  useEffect(() => {
    const flush = () => {
      const md = editorRef.current?.getMarkdown();
      if (md !== undefined) setDraft(draftKey, md);
    };
    const onVis = () => { if (document.visibilityState === "hidden") flush(); };
    document.addEventListener("visibilitychange", onVis);
    window.addEventListener("pagehide", flush);
    return () => {
      document.removeEventListener("visibilitychange", onVis);
      window.removeEventListener("pagehide", flush);
    };
  }, [draftKey, setDraft]);

  // Await-then-render send (MUL-5181): the shared hook reads the markdown,
  // guards empty/in-flight, re-checks the upload gate, locks + spins via
  // `submitting`, and clears only once the server accepts — a failed send keeps
  // the draft instead of silently dropping it.
  // Stale-submit guard (MUL-5181 P0): if this composer unmounts mid-submit
  // (issue detail closed) and the user reopens and types a new draft under the
  // same key, the late success may only clear the draft it submitted.
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);
  const submittedEntryRef = useRef<unknown>(null);
  // Set by `onAccepted` only on the branch that actually wiped the editor; read
  // back by `afterAccepted`. Bound this way because the stale-submit guard can
  // leave a mid-flight draft in place: dropping the caret then would yank it
  // out of the sentence the user is still typing.
  const editorScrubbedRef = useRef(false);
  const acceptedCommentIdRef = useRef<string | null>(null);

  const { submitting, submit } = useComposerSubmit({
    editorRef,
    uploadGate: gate,
    normalize: (raw) => {
      const current = useCommentDraftStore.getState().getAnnotations(draftKey);
      return hasReplyIntent(raw, current) ? composeAnnotatedReply(raw, current) : "";
    },
    // A top-level comment ends a turn: the caret is dropped rather than kept,
    // so the composer stops reading as "still writing" once the comment is
    // posted above it. Thread replies are the opposite — see ReplyInput.
    afterAccepted: () => (editorScrubbedRef.current ? "blur" : "none"),
    onSubmit: async (content) => {
      setSendFailed(false);
      if (sendDisabled || (!localCommand && (!previewReady || commentTriggerPreviewSignature(content) !== commentTriggerPreviewSignature(composedContent)))) return false;
      editorScrubbedRef.current = false;
      const { suppressAgentIds, steerTaskIds } = routing;
      // A preview still catching up with an edited @mention can name a
      // recipient this comment no longer addresses: never stop a run on it.
      const restartTaskIds = !localCommand && triggerPreview.isCurrent ? routing.restartTaskIds : [];
      if (!(await stopRunsBeforeSend(restartTaskIds))) return false;
      // Flush the editor's pending debounce before snapshotting — a late flush
      // of pre-submit typing must not read as an edit made during the request.
      const pending = editorRef.current?.flushPendingUpdate?.();
      if (pending != null) setDraft(draftKey, pending);
      submittedEntryRef.current = useCommentDraftStore.getState().drafts[draftKey];
      // Bind only uploads the BODY still references (MUL-5181): deleting an
      // inline image really unbinds it. Uploads that finished after a close
      // are written back into the body by the settle handler, so surviving
      // files are referenced too — never silently attached.
      const activeIds = pendingAttachments
        .filter((a) => contentReferencesAttachment(content, a))
        .map((a) => a.id);
      return onSubmit(
        content,
        activeIds.length > 0 ? activeIds : undefined,
        suppressAgentIds.length > 0 ? suppressAgentIds : undefined,
        steerTaskIds.length > 0 ? steerTaskIds : undefined,
      ).then((commentId) => {
        acceptedCommentIdRef.current = typeof commentId === "string" ? commentId : null;
        setSendFailed(!commentId);
        return !!commentId;
      }).catch(() => { setSendFailed(true); return false; });
    },
    onAccepted: () => {
      // Success may only consume the entry it submitted (MUL-5181 P0): edits
      // made while the request was in flight — or by a reopened composer after
      // this one unmounted — survive both in the store and in the editor.
      // Flush the pending debounce first so typing still inside the window is
      // judged correctly (a no-op flush preserves entry identity).
      const lateMd = editorRef.current?.flushPendingUpdate?.();
      if (lateMd != null) setDraft(draftKey, lateMd);
      const store = useCommentDraftStore.getState();
      const live = store.drafts[draftKey];
      const untouched = live === undefined || live === submittedEntryRef.current;
      let clearFailed = false;
      if (untouched) {
        try { store.clearDraft(draftKey); } catch { clearFailed = true; }
      }
      if (!mountedRef.current || !untouched) return;
      editorRef.current?.clearContent();
      setContent("");
      setIsEmpty(true);
      setExpanded(false);
      setDraftState(clearFailed ? "clear_failed" : null);
      resetRecipients();
      editorScrubbedRef.current = true;
      if (acceptedCommentIdRef.current) onAccepted?.(acceptedCommentIdRef.current);
    },
  });

  const collapse = () => {
    editorRef.current?.flushPendingUpdate?.();
    setExpanded(false);
    requestAnimationFrame(() => shellRef.current?.focus());
  };

  return (
    <div
      {...dropZoneProps}
      className="relative flex min-h-0 flex-col overflow-y-auto rounded-lg bg-card ring-1 ring-border"
      onKeyDown={(event) => {
        if (event.key !== "Escape" || event.defaultPrevented || event.nativeEvent.isComposing) return;
        event.preventDefault();
        event.stopPropagation();
        collapse();
      }}
    >
      {canSend && !localCommand && <div className="border-b px-3 py-2 text-caption" aria-live="polite">
        {!previewReady ? <div className="flex flex-wrap items-center gap-2">
          <span>{t(($) => previewError ? $.composer_ui.preview_failed : $.composer_ui.preview_loading)}</span>
          {previewError && <Button size="sm" variant="outline" onClick={() => { if (triggerPreview.isError) triggerPreview.retry(); if (recipientsError) retryRecipients(); }}>{t(($) => $.release_readiness.retry)}</Button>}
        </div> : <>
          <CommentTriggerChips compact={compact} recipients={recipients} blocked={triggerPreview.blocked} hasAllMembersMention={triggerPreview.hasAllMembersMention} draftContent={composedContent} onActionChange={setAction} />
          {recipients.every((entry) => entry.action === "skip") && <p>{t(($) => $.composer_ui.comment_only)}</p>}
        </>}
      </div>}
      {draftState && <p role="status" className={cn("px-3 pt-2 text-caption", draftState.endsWith("failed") ? "text-destructive" : "text-muted-foreground")}>{t(($) => $.composer_ui.draft[draftState])}</p>}
      {sendFailed && <p role="alert" className="px-3 pt-2 text-caption text-destructive">{t(($) => $.comment.send_failed)}</p>}
      {attachmentsBlockSteer && <div className="px-3 pt-2">
        <SteerAttachmentNotice />
      </div>}
      {annotations.length > 0 && <div className="px-3 pt-2">
        <ReplyAnnotations draftKey={draftKey} annotations={annotations} disabled={submitting}
          onEditAnnotation={onEditAnnotation} />
      </div>}
      {/* Lock the editor while the send is in flight. ContentEditor can't
          toggle Tiptap's `editable` post-mount (see its docstring), so the
          documented way to make it non-interactive is a pointer-events-none +
          dimmed wrapper. */}
      {lazy.active && (
      <div
        className={cn(
          "flex-1 min-h-[88px] overflow-y-auto px-3 py-2",
          // Pinned to the viewport bottom the composer grows upward; cap it
          // so a long draft can't swallow the whole timeline (the editor
          // area scrolls internally instead).
          sticky && "max-h-[40vh]",
          submitting && "pointer-events-none opacity-60",
          (!lazy.ready || !expanded) && "hidden",
        )}
        aria-busy={submitting || undefined}
      >
        <ContentEditor
          ref={editorRef}
          value={persistedDraft ?? content}
          onReady={lazy.onReady}
          placeholder={placeholder}
          ariaLabel={inputLabel ?? t(($) => $.composer_ui.input_label)}
          onUpdate={(md) => {
            setContent(md);
            setIsEmpty(!md.trim());
            // Debounced upstream (debounceMs=100). Persist on every tick so a
            // reload or scroll-out-of-viewport restores work to the keystroke.
            // setDraft keeps any pending attachments and drops the entry only
            // when text AND attachments are both empty.
            setDraft(draftKey, md);
          }}
          onSubmit={submit}
          onEscape={collapse}
          onUploadFile={textOnly ? undefined : uploadToDestination}
          onUploadingChange={uploadGate.onUploadingChange}
          debounceMs={100}
          currentIssueId={issueId}
          attachments={pendingAttachments}
          enableSlashCommands={!textOnly}
          slashCommandMode="command"
          quickActionMenu={quickActionMenu}
        />
      </div>
      )}
      {/* Static shell — visually clones the empty single-line composer.
          Real editor mounts (hidden) on first intent; shell stays visible
          until it's ready so the card never blanks or shifts. */}
      {(!lazy.ready || !expanded) && (
        <button
          ref={shellRef}
          type="button"
          data-testid="comment-composer-shell"
          aria-label={placeholder}
          className="flex-1 min-h-0 cursor-text px-3 py-2 text-left"
          onClick={() => { setExpanded(true); lazy.activate(); if (lazy.ready) requestAnimationFrame(() => editorRef.current?.focus()); }}

        >
          {/* rich-text-editor + <p>: the shell line inherits the editor's
              exact type metrics (line-height 1.625 from prose.css), so the
              shell→editor swap doesn't shift layout. */}
          <div className="rich-text-editor text-body">
            <p className="text-muted-foreground">{content.trim() ? t(($) => $.composer_ui.resume_draft) : placeholder}</p>
          </div>
        </button>
      )}
      <div className="sticky bottom-0 z-10 flex shrink-0 flex-wrap items-center justify-end gap-2 bg-card px-3 py-2">
        {expanded && !textOnly && <FileUploadButton size="sm" multiple onSelect={(file) => lazy.uploadOrQueue([file])} />}
        {expanded && sendShortcut && <span className="mr-auto text-caption text-muted-foreground"><ShortcutKeycaps shortcut={sendShortcut} decorative /> {t(($) => $.comment.send_tooltip)} · {t(($) => $.composer_ui.collapse_hint)}</span>}
        {pauseControl && <Button size="icon-sm" variant="outline" aria-label={t(($) => $.task_workspace.pause_task)} title={t(($) => $.task_workspace.pause_scope)} disabled={pauseControl.pending} onClick={pauseControl.onPause} aria-busy={pauseControl.pending}>
          {pauseControl.pending ? <Loader2 className="animate-spin" /> : <Square />}
        </Button>}
        {(!pauseControl || canSend) && <Button
          size={compact ? "icon-sm" : "default"}
          aria-label={submitLabel ?? (compact ? t(($) => !recipients.some((entry) => entry.action !== "skip") ? $.composer_ui.post_comment : $.composer_ui.send_instruction) : undefined)}
          variant={routing.restartTaskIds.length > 0 ? "destructive" : "default"}
          onClick={submit}
          disabled={sendDisabled || !canSend || submitting || gate.uploading || (!localCommand && !previewReady)}
          aria-busy={submitting || gate.uploading || undefined}
        >
          {submitting && <Loader2 className="animate-spin motion-reduce:animate-none" aria-hidden="true" />}
          {compact ? (!submitting && <ArrowUp />) : gate.uploading ? tEditor(($) => $.upload.in_progress) : routing.restartTaskIds.length > 0 ? t(($) => $.comment.restart_send)
            : t(($) => !recipients.some((entry) => entry.action !== "skip") ? $.composer_ui.post_comment
              : recipients.some((entry) => entry.action !== "skip" && (entry.action === "after_run" || entry.state.kind === "queued")) ? $.composer_ui.send_queue : $.composer_ui.send_instruction)}
        </Button>}
      </div>
      {isDragOver && <FileDropOverlay />}
    </div>
  );
}

export { CommentInput };
