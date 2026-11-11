"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  credentialsOptions,
  useSaveWorkbenchCredential,
} from "@multica/core/workbench/configuration";
import {
  useSaveProjectDeployment,
  useCheckDeploymentResource,
  useCheckDeploymentResourceDraft,
  useDiscoverDeploymentHost,
  type DeploymentResource,
  type ProjectDeployment,
} from "@multica/core/workbench/application-deployment";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../../i18n";
import { ConfigurationSelect } from "./configuration-select";

const sshHostFingerprintCommand =
  "ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub -E sha256";

export function DeploymentResourceCheck({
  projectId,
  resourceId,
}: {
  projectId: string;
  resourceId: string;
}) {
  const { t } = useT("projects");
  const check = useCheckDeploymentResource(projectId, resourceId);
  return (
    <div className="space-y-2">
      <Button
        variant="outline"
        disabled={check.isPending}
        onClick={() => check.mutate()}
      >
        {t(($) => $.application_deployment.check_ssh)}
      </Button>
      {check.isPending && (
        <p role="status">{t(($) => $.application_deployment.checking)}</p>
      )}
      {check.data && (
        <p
          role="status"
          className={
            check.data.ok ? "text-caption" : "text-caption text-destructive"
          }
        >
          {check.data.ok
            ? t(($) => $.application_deployment.ssh_ready)
            : check.data.detail}
        </p>
      )}
      {check.isError && (
        <p role="alert" className="text-destructive">
          {check.error.message}
        </p>
      )}
    </div>
  );
}

export function DeploymentResourceFields({
  projectId,
  resource,
  onChange: onResourceChange,
}: {
  projectId: string;
  resource: DeploymentResource;
  onChange: (value: DeploymentResource) => void;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const credentials = useQuery(credentialsOptions(wsId));
  const saveCredential = useSaveWorkbenchCredential(wsId);
  const discover = useDiscoverDeploymentHost(projectId);
  const connection = useCheckDeploymentResourceDraft(projectId);
  const currentCheck =
    JSON.stringify(connection.variables) === JSON.stringify(resource);
  const onChange = (value: DeploymentResource) => {
    connection.reset();
    onResourceChange(value);
  };

  const authMethod = resource.auth_method ?? "key";
  const [creatingCredential, setCreatingCredential] = useState(false);
  const [credentialName, setCredentialName] = useState("");
  const [credentialSecret, setCredentialSecret] = useState("");
  const [credentialError, setCredentialError] = useState("");
  const [readingKey, setReadingKey] = useState(false);
  const credentialBusy = saveCredential.isPending || readingKey;
  const detectedHost =
    discover.variables?.host === resource.host &&
    discover.variables?.port === resource.port
      ? discover.data?.host_key
      : undefined;
  const availableCredentials = (credentials.data?.credentials ?? []).filter(
    (item) =>
      item.kind === (authMethod === "password" ? "ssh-password" : "ssh-key") &&
      item.usage.includes("deploy") &&
      (!item.project_id || item.project_id === projectId),
  );
  const cancelCredential = () => {
    setCreatingCredential(false);
    setCredentialSecret("");
    setCredentialName("");
    setCredentialError("");
    saveCredential.reset();
  };
  const readPrivateKey = async (file?: File) => {
    if (!file) return;
    setCredentialError("");
    if (file.size > 65536) {
      setCredentialError(t(($) => $.application_deployment.private_key_size));
      return;
    }
    setReadingKey(true);
    try {
      setCredentialSecret(await file.text());
    } catch {
      setCredentialError(
        t(($) => $.application_deployment.private_key_read_failed),
      );
    } finally {
      setReadingKey(false);
    }
  };
  const createCredential = async () => {
    setCredentialError("");
    try {
      const saved = await saveCredential.mutateAsync({
        data: {
          label: credentialName.trim(),
          kind: authMethod === "password" ? "ssh-password" : "ssh-key",
          project_id: projectId,
          usage: ["deploy"],
          secret: credentialSecret,
          expires_at: null,
        },
      });
      onChange({ ...resource, credential_id: saved.id });
      cancelCredential();
    } catch (cause) {
      setCredentialError(
        cause instanceof Error ? cause.message : String(cause),
      );
      saveCredential.reset();
    }
  };
  return (
    <>
      <ConfigurationSelect
        label={t(($) => $.application_deployment.login_method)}
        value={authMethod}
        disabled={credentialBusy}
        items={[
          {
            value: "password",
            label: t(($) => $.application_deployment.password_login),
          },
          { value: "key", label: t(($) => $.application_deployment.key_login) },
        ]}
        onChange={(value) => {
          cancelCredential();
          onChange({
            ...resource,
            auth_method: value === "password" ? "password" : "key",
            credential_id: "",
          });
        }}
      />
      {(["name", "host", "port", "username"] as const).map((key) => (
        <label key={key} className="block space-y-1">
          {t(($) => $.application_deployment[key])}
          <Input
            required
            disabled={credentialBusy || discover.isPending}
            value={resource[key]}
            onChange={(e) =>
              onChange({
                ...resource,
                [key]: e.target.value,
                ...(key === "host" || key === "port" ? { host_key: "" } : {}),
              })
            }
          />
        </label>
      ))}
      <section className="space-y-2 rounded-lg border p-3">
        <p className="text-label">
          {t(($) => $.application_deployment.server_identity)}
        </p>
        {resource.host_key && (
          <p className="break-all text-caption">
            {t(($) => $.application_deployment.identity_confirmed)} ·{" "}
            {resource.host_key}
          </p>
        )}
        <Button
          type="button"
          variant="outline"
          disabled={
            !resource.host.trim() ||
            !resource.port ||
            discover.isPending ||
            credentialBusy
          }
          onClick={() =>
            discover.mutate({ host: resource.host, port: resource.port })
          }
        >
          {t(($) => $.application_deployment.detect_server)}
        </Button>
        {discover.isPending && (
          <p role="status">{t(($) => $.application_deployment.checking)}</p>
        )}
        {discover.isError && (
          <p role="alert" className="text-destructive">
            {discover.error.message}
          </p>
        )}
        {detectedHost && detectedHost !== resource.host_key && (
          <div className="space-y-2">
            {resource.host_key && (
              <p role="alert" className="text-destructive">
                {t(($) => $.application_deployment.identity_changed)}
              </p>
            )}
            <code className="block break-all">{detectedHost}</code>
            <p className="text-caption text-muted-foreground">
              {t(($) => $.application_deployment.identity_verify)}
            </p>
            <Button
              type="button"
              onClick={() => onChange({ ...resource, host_key: detectedHost })}
            >
              {t(($) => $.application_deployment.confirm_identity)}
            </Button>
          </div>
        )}
        {detectedHost && detectedHost === resource.host_key && (
          <p role="status">
            {t(($) => $.application_deployment.identity_matches)}
          </p>
        )}
        <details className="text-caption">
          <summary className="cursor-pointer">
            {t(($) => $.application_deployment.manual_fingerprint)}
          </summary>
          <div className="mt-2 space-y-2">
            <p>{t(($) => $.application_deployment.host_key_help)}</p>
            <p>{t(($) => $.application_deployment.host_key_command)}</p>
            <code className="block break-all">{sshHostFingerprintCommand}</code>
            <label className="block space-y-1">
              {t(($) => $.application_deployment.host_key)}
              <Input
                value={resource.host_key}
                disabled={discover.isPending || credentialBusy}
                onChange={(event) =>
                  onChange({ ...resource, host_key: event.target.value })
                }
              />
            </label>
          </div>
        </details>
      </section>
      <ConfigurationSelect
        label={t(($) => $.application_deployment.credential)}
        disabled={credentialBusy}
        value={resource.credential_id}
        onChange={(value) => onChange({ ...resource, credential_id: value })}
        items={[
          { value: "", label: t(($) => $.application_deployment.choose) },
          ...availableCredentials.map((item) => ({
            value: item.id,
            label: item.label,
          })),
        ]}
      />
      {credentials.data && !availableCredentials.length && (
        <p className="text-caption text-muted-foreground">
          {t(($) => $.application_deployment.no_ssh_credentials)}
        </p>
      )}
      {credentials.data && !credentials.data.encryption_configured && (
        <p role="alert" className="text-caption text-warning">
          {t(($) => $.workbench.encryption_missing)}
        </p>
      )}
      {creatingCredential ? (
        <fieldset
          disabled={credentialBusy}
          className="space-y-3 rounded-lg border p-3"
        >
          <legend className="px-1 text-label">
            {t(($) => $.application_deployment.add_ssh_credential)}
          </legend>
          <label className="block space-y-1">
            {t(($) => $.application_deployment.credential_name)}
            <Input
              value={credentialName}
              maxLength={120}
              onChange={(event) => setCredentialName(event.target.value)}
            />
          </label>
          {authMethod === "key" && (
            <label className="block space-y-1">
              {t(($) => $.application_deployment.private_key_file)}
              <Input
                type="file"
                onChange={(event) =>
                  void readPrivateKey(event.target.files?.[0])
                }
              />
            </label>
          )}
          <label className="block space-y-1">
            {authMethod === "password"
              ? t(($) => $.application_deployment.login_password)
              : t(($) => $.application_deployment.private_key)}
            {authMethod === "password" ? (
              <Input
                type="password"
                value={credentialSecret}
                autoComplete="new-password"
                onChange={(event) => setCredentialSecret(event.target.value)}
              />
            ) : (
              <Textarea
                value={credentialSecret}
                onChange={(event) => setCredentialSecret(event.target.value)}
                autoComplete="off"
                spellCheck={false}
                className="min-h-32 font-mono"
              />
            )}
          </label>
          <p className="text-caption text-muted-foreground">
            {authMethod === "password"
              ? t(($) => $.application_deployment.password_help)
              : t(($) => $.application_deployment.private_key_help)}
          </p>
          {credentialError && (
            <p role="alert" className="text-destructive">
              {credentialError}
            </p>
          )}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={cancelCredential}>
              {t(($) => $.releases.cancel)}
            </Button>
            <Button
              type="button"
              disabled={
                !credentialName.trim() ||
                !credentialSecret.trim() ||
                credentialBusy
              }
              onClick={() => void createCredential()}
            >
              {t(($) => $.application_deployment.save_credential)}
            </Button>
          </div>
        </fieldset>
      ) : (
        <Button
          type="button"
          variant="outline"
          disabled={!credentials.data?.encryption_configured}
          onClick={() => {
            setCredentialName(resource.name);
            setCreatingCredential(true);
          }}
        >
          {t(($) => $.application_deployment.add_ssh_credential)}
        </Button>
      )}
      {credentials.isError && <p role="alert">{credentials.error.message}</p>}
      <div className="space-y-2">
        <Button
          type="button"
          variant="outline"
          disabled={
            credentialBusy ||
            discover.isPending ||
            connection.isPending ||
            !resource.name.trim() ||
            !resource.host.trim() ||
            !resource.port.trim() ||
            !resource.username.trim() ||
            !resource.host_key ||
            !resource.credential_id
          }
          aria-busy={connection.isPending}
          onClick={() => connection.mutate({ ...resource })}
        >
          {t(($) => $.application_deployment.check_ssh)}
        </Button>
        {currentCheck && connection.isPending && (
          <p role="status">{t(($) => $.application_deployment.checking)}</p>
        )}
        {currentCheck && connection.data && (
          <p
            role="status"
            className={connection.data.ok ? "text-caption" : "text-caption text-destructive"}
          >
            {connection.data.ok
              ? t(($) => $.application_deployment.ssh_ready)
              : connection.data.detail}
          </p>
        )}
        {currentCheck && connection.isError && (
          <p role="alert" className="text-destructive">{connection.error.message}</p>
        )}
      </div>
    </>
  );
}

export function DeploymentResourceDialog({
  projectId,
  initial,
  data,
  onClose,
}: {
  projectId: string;
  initial: DeploymentResource;
  data: ProjectDeployment;
  onClose: () => void;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const [resource, setResource] = useState(initial);
  const save = useSaveProjectDeployment(wsId, projectId);
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    try {
      await save.mutateAsync({
        ...data,
        config: {
          ...data.config,
          resources: [
            ...data.config.resources.filter((item) => item.id !== resource.id),
            resource,
          ],
        },
      });
      onClose();
    } catch {
      return;
    }
  };
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) onClose();
      }}
    >
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {t(($) => $.application_deployment.resources)}
          </DialogTitle>
        </DialogHeader>
        <form className="space-y-4" onSubmit={submit}>
          <DeploymentResourceFields
            projectId={projectId}
            resource={resource}
            onChange={setResource}
          />
          {save.isError && (
            <p role="alert" className="text-destructive">
              {save.error.message}
            </p>
          )}
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={save.isPending}
            >
              {t(($) => $.releases.cancel)}
            </Button>
            <Button
              type="submit"
              disabled={
                save.isPending || !resource.credential_id || !resource.host_key
              }
            >
              {t(($) => $.application_deployment.save)}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
