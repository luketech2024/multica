"use client";

import { useState } from "react";
import { Button } from "@multica/ui/components/ui/button";
import { useAnswerUserInput } from "@multica/core/issues/user-input";
import type { TaskUserInput, UserInputResponse } from "@multica/core/issues/user-input-schema";
import { useCommentDraftStore } from "@multica/core/issues/stores/comment-draft-store";
import { useAuthStore } from "@multica/core/auth";
import { CommentInput } from "./comment-input";
import { conversationCommand } from "@multica/core/workbench/configuration";
import { useT } from "../../i18n";

export function UserInputMessages({ requests, selectedId, onSelect }: {
  requests: TaskUserInput[]; selectedId?: string; onSelect: (id: string) => void;
}) {
  const { t } = useT("issues");
  const user = useAuthStore((state) => state.user);
  return <>{requests.map((request) => <div key={request.id} className="my-4 space-y-2" data-testid="user-input-message">
    {request.request.questions.map((question) => <div key={question.id}>
      <p className="whitespace-pre-wrap text-body-sm">{question.question}</p>
      {request.response?.answers[question.id] && <p data-message-side={request.answered_by === user?.id ? "right" : "left"} className={`mt-2 w-fit max-w-[85%] whitespace-pre-wrap rounded-lg bg-muted px-3 py-2 text-body-sm ${request.answered_by === user?.id ? "ml-auto" : "mr-auto"}`}>{request.response.answers[question.id]!.answers.join("\n")}</p>}
    </div>)}
    {request.status === "pending" ? selectedId === request.id ? <p className="text-caption text-muted-foreground">{t(($) => $.conversation.waiting)}</p> : <Button size="sm" variant="ghost" onClick={() => onSelect(request.id)}>{t(($) => $.conversation.answer)}</Button>
      : <p className="text-caption text-muted-foreground">{t(($) => request.status === "resolved" ? $.conversation.answered : request.status === "answered" ? $.conversation.answer_sent : $.conversation.question_closed)}</p>}
  </div>)}</>;
}

export function UserInputComposer({ wsId, request, pauseControl }: {
  wsId: string; request: TaskUserInput; pauseControl?: { pending: boolean; onPause: () => Promise<boolean> };
}) {
  const { t } = useT("issues");
  const prefix = `input:${request.id}`;
  const answerKey = (id: string) => `reply:${request.issue_id}:${prefix}:answer:${id}` as const;
  const drafts = useCommentDraftStore((state) => state.drafts);
  const questions = request.request.questions;
  const [index, setIndex] = useState(() => {
    const unanswered = questions.findIndex((question) => !drafts[answerKey(question.id)]?.content);
    return unanswered < 0 ? questions.length - 1 : unanswered;
  });
  const question = questions[index]!;
  const threadId = `${prefix}:draft:${question.id}`;
  const mutation = useAnswerUserInput(wsId, request.issue_id, request.id);
  const [error, setError] = useState<string>();
  const submit = async (content: string, attachments?: string[]) => {
    setError(undefined);
    if (conversationCommand(content) === "pause" && pauseControl && !attachments?.length) return pauseControl.onPause();
    if (attachments?.length || (question.options.length > 0 && !question.isOther && !question.options.some((option) => option.label === content))) {
      setError(t(($) => $.conversation.select_option));
      return false;
    }
    useCommentDraftStore.getState().setDraft(answerKey(question.id), content);
    const answers: UserInputResponse["answers"] = {};
    for (const item of questions) {
      const answer = useCommentDraftStore.getState().getDraft(answerKey(item.id));
      if (!answer) { setIndex(questions.indexOf(item)); return true; }
      answers[item.id] = { answers: [answer] };
    }
    try {
      await mutation.mutateAsync({ answers });
      for (const item of questions) useCommentDraftStore.getState().clearDraft(answerKey(item.id));
      return true;
    } catch {
      setError(t(($) => $.conversation.answer_failed));
      return false;
    }
  };
  return <div className="space-y-2" data-testid="user-input-composer">
    <p className="text-caption">{t(($) => $.conversation.answer)} · {question.header || question.question} · {index + 1}/{questions.length}</p>
    {questions.length > 1 && <div className="flex flex-wrap gap-1">{questions.map((item, i) => <Button key={item.id} size="xs" variant={index === i ? "secondary" : "ghost"} disabled={mutation.isPending} onClick={() => setIndex(i)}>{item.header || String(i + 1)}</Button>)}</div>}
    <div className="flex flex-wrap gap-2">{question.options.map((option) => <Button key={option.label} size="sm" variant="outline" disabled={mutation.isPending} title={option.description} onClick={() => useCommentDraftStore.getState().setDraft(`reply:${request.issue_id}:${threadId}`, option.label)}>{option.label}</Button>)}</div>
    {error && <p role="alert" className="text-caption text-destructive">{error}</p>}
    <CommentInput key={threadId} issueId={request.issue_id} parentId={request.id} draftThreadId={threadId} compact docked localCommand textOnly inputLabel={t(($) => $.conversation.answer)} submitLabel={t(($) => $.conversation.answer)} sendDisabled={mutation.isPending || pauseControl?.pending} pauseControl={pauseControl} onSubmit={submit} />
  </div>;
}
