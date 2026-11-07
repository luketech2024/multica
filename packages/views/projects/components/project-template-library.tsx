"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  businessTemplatesOptions,
  usePublishBusinessTemplate,
  useSaveProjectTemplate,
  mergeBusinessTemplateFields,
  templateMergeFields,
  type BusinessTemplateAsset,
  type ProjectTemplate,
  type TemplateMergeField,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@multica/ui/components/ui/dialog";
import { useT } from "../../i18n";
import { BusinessTemplateEditor } from "./business-template-editor";
import { BusinessTemplateContent } from "./business-template-content";

export function ProjectTemplateLibrary({ projectId, template }: {
  projectId: string;
  template: ProjectTemplate;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const catalog = useQuery(businessTemplatesOptions(wsId));
  const publish = usePublishBusinessTemplate(wsId);
  const [editing, setEditing] = useState<{ asset?: BusinessTemplateAsset }>();
  const [merging, setMerging] = useState<BusinessTemplateAsset>();
  const [published, setPublished] = useState(false);
  const source = catalog.data?.templates.find((asset) => asset.id === template.source_id);
  return <div className="space-y-2">
    <div className="flex flex-wrap gap-2">
      <Button variant="outline" size="sm" onClick={() => { setPublished(false); setEditing({}); }}>{t(($) => $.template_library.copy)}</Button>
      {source && !source.builtin && <Button variant="outline" size="sm" onClick={() => { setPublished(false); setEditing({ asset: source }); }}>{t(($) => $.template_library.edit_source)}</Button>}
      {source && source.revision > template.source_revision && <Button variant="outline" size="sm" onClick={() => setMerging(source)}>{t(($) => $.template_library.update, { version: source.revision })}</Button>}
    </div>
    {published && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.template_library.saved)}</p>}
    {catalog.isError && <Button variant="ghost" size="sm" onClick={() => void catalog.refetch()}>{t(($) => $.workbench.retry)}</Button>}
    <Dialog open={!!editing} onOpenChange={(open) => { if (!open && !publish.isPending) setEditing(undefined); }}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-3xl">
        <DialogHeader><DialogTitle>{editing?.asset ? t(($) => $.template_library.edit_source) : t(($) => $.template_library.copy)}</DialogTitle></DialogHeader>
        <p className="text-caption text-muted-foreground">{t(($) => $.template_library.publish_hint)}</p>
        {editing && <BusinessTemplateEditor initial={editing.asset?.content ?? template.content} pending={publish.isPending} onCancel={() => setEditing(undefined)} onSave={async (content) => {
          await publish.mutateAsync({ content, id: editing.asset?.id, revision: editing.asset?.revision });
          setEditing(undefined);
          setPublished(true);
        }} />}
      </DialogContent>
    </Dialog>
    {merging && <TemplateMergeDialog projectId={projectId} template={template} incoming={merging} onClose={() => setMerging(undefined)} />}
  </div>;
}

function TemplateMergeDialog({ projectId, template, incoming, onClose }: {
  projectId: string;
  template: ProjectTemplate;
  incoming: BusinessTemplateAsset;
  onClose: () => void;
}) {
  const { t } = useT("projects");
  const save = useSaveProjectTemplate(useWorkspaceId(), projectId);
  const [fields, setFields] = useState<TemplateMergeField[]>([]);
  const [error, setError] = useState<string>();
  const labels = {
    name: t(($) => $.templates.name),
    background: t(($) => $.templates.background),
    audience: t(($) => $.templates.audience),
    scope_out: t(($) => $.templates.scope_out),
    ac_skeleton: t(($) => $.templates.acceptance),
    forbidden: t(($) => $.templates.forbidden),
    dod: t(($) => $.templates.dod),
    resources: t(($) => $.templates.resources),
    terms: t(($) => $.templates.terms),
  };
  const changed = templateMergeFields.filter((field) => JSON.stringify(template.content[field]) !== JSON.stringify(incoming.content[field]));
  return <Dialog open onOpenChange={(open) => { if (!open && !save.isPending) onClose(); }}>
    <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-4xl">
      <DialogHeader><DialogTitle>{t(($) => $.template_library.update, { version: incoming.revision })}</DialogTitle></DialogHeader>
      <div className="grid gap-4 sm:grid-cols-2">
        <section className="space-y-2"><h3 className="font-medium">{t(($) => $.template_library.current)}</h3><p>{template.content.name}</p><BusinessTemplateContent template={template.content} /></section>
        <section className="space-y-2"><h3 className="font-medium">{t(($) => $.template_library.incoming)}</h3><p>{incoming.content.name}</p><BusinessTemplateContent template={incoming.content} /></section>
      </div>
      <fieldset disabled={save.isPending} className="space-y-3">
        <legend className="mb-2 text-label font-medium">{t(($) => $.template_library.choose)}</legend>
        <div className="grid gap-2 sm:grid-cols-2">
          {changed.map((field) => (
            <label key={field} className="flex items-center gap-2">
              <Checkbox
                checked={fields.includes(field)}
                onCheckedChange={(checked) => setFields((selected) =>
                  checked ? [...selected, field] : selected.filter((value) => value !== field),
                )}
              />
              {labels[field]}
            </label>
          ))}
        </div>
        {error && <p role="alert" className="text-destructive">{error}</p>}
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>{t(($) => $.workbench.cancel)}</Button>
          <Button onClick={async () => {
            setError(undefined);
            try {
              await save.mutateAsync({ ...template, content: mergeBusinessTemplateFields(template.content, incoming.content, fields), mergeSourceRevision: incoming.revision });
              onClose();
            } catch (cause) {
              setError(cause instanceof Error ? cause.message : t(($) => $.workbench.load_failed));
            }
          }}>{fields.length ? t(($) => $.template_library.merge) : t(($) => $.template_library.keep)}</Button>
        </div>
      </fieldset>
    </DialogContent>
  </Dialog>;
}
