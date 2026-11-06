"use client";

import { useState } from "react";
import type { BusinessTemplate } from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";

function TemplateRows({
  labels,
  values,
  onChange,
}: {
  labels: string[];
  values: string[][];
  onChange: (values: string[][]) => void;
}) {
  const { t } = useT("projects");
  const [keys, setKeys] = useState(() => values.map(() => crypto.randomUUID()));
  return (
    <div className="space-y-2">
      {values.map((row, rowIndex) => (
        <div key={keys[rowIndex]} className="flex flex-wrap items-end gap-2">
          {labels.map((label, column) => (
            <label key={label} className="min-w-32 flex-1 space-y-1">
              <span>{label}</span>
              <Input
                value={row[column] ?? ""}
                onChange={(event) =>
                  onChange(
                    values.map((value, index) =>
                      index === rowIndex
                        ? value.map((cell, cellIndex) =>
                            cellIndex === column ? event.target.value : cell,
                          )
                        : value,
                    ),
                  )
                }
              />
            </label>
          ))}
          <Button
            type="button"
            variant="ghost"
            size="sm"
            aria-label={`${t(($) => $.workbench.remove)} ${rowIndex + 1}`}
            onClick={() => {
              setKeys(keys.filter((_, index) => index !== rowIndex));
              onChange(values.filter((_, index) => index !== rowIndex));
            }}
          >
            {t(($) => $.workbench.remove)}
          </Button>
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={values.length >= 100}
        onClick={() => {
          setKeys([...keys, crypto.randomUUID()]);
          onChange([...values, labels.map(() => "")]);
        }}
      >
        {t(($) => $.workbench.add)}
      </Button>
    </div>
  );
}

export function BusinessTemplateEditor({
  initial,
  pending,
  onSave,
  onCancel,
}: {
  initial: BusinessTemplate;
  pending: boolean;
  onSave: (content: BusinessTemplate) => Promise<void>;
  onCancel: () => void;
}) {
  const { t } = useT("projects");
  const [content, setContent] = useState(initial);
  const [terms, setTerms] = useState<string[][]>(() =>
    Object.entries(initial.terms),
  );
  const [error, setError] = useState<string>();
  const fields = [
    { key: "background", label: t(($) => $.templates.background) },
    { key: "audience", label: t(($) => $.templates.audience) },
    { key: "scope_out", label: t(($) => $.templates.scope_out) },
    { key: "ac_skeleton", label: t(($) => $.templates.acceptance) },
    { key: "dod", label: t(($) => $.templates.dod) },
  ] as const;
  const submit = async () => {
    setError(undefined);
    if (
      new Set(terms.map(([term = ""]) => term.trim())).size !== terms.length
    ) {
      setError(t(($) => $.templates.duplicate_term));
      return;
    }
    try {
      await onSave({
        ...content,
        terms: Object.fromEntries(
          terms.map(([term = "", meaning = ""]) => [term.trim(), meaning]),
        ),
      });
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : t(($) => $.workbench.load_failed),
      );
    }
  };
  return (
    <fieldset disabled={pending} className="space-y-4 text-caption">
      <label className="block space-y-1">
        <span>{t(($) => $.templates.name)}</span>
        <Input
          value={content.name}
          onChange={(event) =>
            setContent({ ...content, name: event.target.value })
          }
        />
      </label>
      <p className="text-muted-foreground">{t(($) => $.templates.line_hint)}</p>
      {fields.map(({ key, label }) => (
        <label key={key} className="block space-y-1">
          <span>{label}</span>
          <Textarea
            value={content[key].join("\n")}
            onChange={(event) =>
              setContent({
                ...content,
                [key]:
                  event.target.value === ""
                    ? []
                    : event.target.value.split("\n"),
              })
            }
          />
        </label>
      ))}
      <fieldset className="space-y-2">
        <legend>{t(($) => $.templates.forbidden)}</legend>
        <TemplateRows
          labels={[t(($) => $.templates.rule), t(($) => $.templates.check)]}
          values={content.forbidden.map(({ rule, check }) => [rule, check])}
          onChange={(values) =>
            setContent({
              ...content,
              forbidden: values.map(([rule = "", check = ""]) => ({
                rule,
                check,
              })),
            })
          }
        />
      </fieldset>
      <fieldset className="space-y-2">
        <legend>{t(($) => $.templates.resources)}</legend>
        <TemplateRows
          labels={[
            t(($) => $.templates.resource_type),
            t(($) => $.templates.resource_ref),
            t(($) => $.templates.version),
          ]}
          values={content.resources.map(({ type, ref, version }) => [
            type,
            ref,
            version,
          ])}
          onChange={(values) =>
            setContent({
              ...content,
              resources: values.map(([type = "", ref = "", version = ""]) => ({
                type,
                ref,
                version,
              })),
            })
          }
        />
      </fieldset>
      <fieldset className="space-y-2">
        <legend>{t(($) => $.templates.terms)}</legend>
        <TemplateRows
          labels={[t(($) => $.templates.term), t(($) => $.templates.meaning)]}
          values={terms}
          onChange={setTerms}
        />
      </fieldset>
      {error && (
        <p role="alert" className="text-destructive">
          {error}
        </p>
      )}
      <div className="flex gap-2">
        <Button type="button" aria-busy={pending} onClick={() => void submit()}>
          {t(($) => $.workbench.save)}
        </Button>
        <Button type="button" variant="outline" onClick={onCancel}>
          {t(($) => $.workbench.cancel)}
        </Button>
      </div>
    </fieldset>
  );
}
