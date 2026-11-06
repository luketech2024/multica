"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  repositoryStackCatalogOptions,
  type RepositoryProfileInput,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

export function RepositoryProfileFields({
  value,
  onChange,
}: {
  value: RepositoryProfileInput;
  onChange: (value: RepositoryProfileInput) => void;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const catalog = useQuery(repositoryStackCatalogOptions(wsId));
  const templateNames: Record<string, string> = {
    "java-backend": t(($) => $.repository_profile.java_backend),
    "java8-legacy": t(($) => $.repository_profile.java8_legacy),
    "python-service": t(($) => $.repository_profile.python_service),
    "ai-app": t(($) => $.repository_profile.ai_app),
    weapp: t(($) => $.repository_profile.weapp),
    "web-frontend": t(($) => $.repository_profile.web_frontend),
    custom: t(($) => $.repository_profile.custom),
  };
  if (catalog.isPending)
    return <p role="status">{t(($) => $.repository_profile.loading)}</p>;
  if (catalog.isError)
    return (
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => void catalog.refetch()}
      >
        {t(($) => $.repository_profile.retry)}
      </Button>
    );
  return (
    <>
      <label className="block">
        {t(($) => $.repository_profile.template)}
        <select
          className="block w-full rounded-md border bg-surface p-2"
          value={value.template}
          onChange={(event) =>
            onChange({ ...value, template: event.target.value })
          }
        >
          {catalog.data.templates.map((template) => (
            <option key={template.id} value={template.id}>
              {templateNames[template.id] ?? template.id}
            </option>
          ))}
        </select>
      </label>
      <fieldset>
        <legend>{t(($) => $.repository_profile.tags)}</legend>
        <div className="flex flex-wrap gap-2">
          {catalog.data.tags.map((tag) => (
            <label key={tag.id} className="flex items-center gap-1">
              <input
                type="checkbox"
                checked={value.tags.includes(tag.id)}
                onChange={(event) =>
                  onChange({
                    ...value,
                    tags: event.target.checked
                      ? [...value.tags, tag.id]
                      : value.tags.filter((id) => id !== tag.id),
                  })
                }
              />
              {tag.id}
            </label>
          ))}
        </div>
      </fieldset>
      <fieldset>
        <legend>{t(($) => $.repository_profile.delivery)}</legend>
        <div className="flex flex-wrap gap-2">
          {catalog.data.deliveries.map((delivery) => (
            <label key={delivery} className="flex items-center gap-1">
              <input
                type="checkbox"
                checked={value.delivery.includes(delivery)}
                onChange={(event) =>
                  onChange({
                    ...value,
                    delivery: event.target.checked
                      ? delivery === "none"
                        ? ["none"]
                        : [
                            ...value.delivery.filter(
                              (value) => value !== "none",
                            ),
                            delivery,
                          ]
                      : value.delivery.filter((value) => value !== delivery),
                  })
                }
              />
              {delivery}
            </label>
          ))}
        </div>
      </fieldset>
    </>
  );
}
