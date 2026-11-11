"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { DeploymentResourceFields } from "./deployment-resource-fields";
import {
  deploymentCertificatesOptions,
  useUploadDeploymentCertificate,
  useSaveProjectDeployment,
  type ProjectDeployment,
  type DeploymentResource,
  type ApplicationDeployment,
} from "@multica/core/workbench/application-deployment";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../../i18n";
import { ConfigurationSelect } from "./configuration-select";

export function ApplicationDeploymentWizard({
  projectId,
  applicationId,
  environment,
  initial,
  onClose,
}: {
  projectId: string;
  applicationId: string;
  environment: "staging" | "prod";
  initial: ProjectDeployment;
  onClose: () => void;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const app = initial.config.applications.find(
    (item) => item.id === applicationId,
  )!;
  const [step, setStep] = useState(0);
  const [data, setData] = useState(initial);
  const [config, setConfig] = useState<ApplicationDeployment>(
    app.environments[environment] ?? {
      kind: app.source_kind === "image" ? "docker" : "nginx",
      nodes: [],
      directory: "/srv/www/" + app.id,
      port: "8080",
      domain: "",
      https: "off",
      certificate_id: "",
      strategy: "serial",
      health_path: "/",
      command: "",
      namespace: "default",
      spa: false,
    },
  );
  const [resource, setResource] = useState<DeploymentResource>();
  const [uploading, setUploading] = useState(false);
  const [certificateName, setCertificateName] = useState("");
  const [certificateFile, setCertificateFile] = useState<File>();
  const [keyFile, setKeyFile] = useState<File>();
  const [error, setError] = useState("");
  const save = useSaveProjectDeployment(wsId, projectId);
  const upload = useUploadDeploymentCertificate(wsId, projectId);

  const certificates = useQuery(deploymentCertificatesOptions(wsId, projectId));
  const certificate = certificates.data?.certificates.find(
    (item) => item.id === config.certificate_id,
  );
  const busy = save.isPending || upload.isPending;
  const steps = [
    t(($) => $.application_deployment.method),
    t(($) => $.application_deployment.location),
    t(($) => $.application_deployment.access),
    t(($) => $.application_deployment.review),
  ];
  const fields = {
    directory: t(($) => $.application_deployment.deploy_directory),
    port: t(($) => $.application_deployment.service_port),
    domain: t(($) => $.application_deployment.domain),
    health_path: t(($) => $.application_deployment.health),
    command: t(($) => $.application_deployment.start_command),
    namespace: t(($) => $.application_deployment.namespace),
  };
  const input = (key: keyof typeof fields, required = false) => (
    <label key={key} className="block space-y-1 text-label">
      {fields[key]}
      <Input
        required={required}
        value={config[key]}
        onChange={(e) => setConfig({ ...config, [key]: e.target.value })}
      />
    </label>
  );
  const saveConfig = async () => {
    setError("");
    try {
      await save.mutateAsync({
        ...data,
        config: {
          ...data.config,
          applications: data.config.applications.map((item) =>
            item.id === applicationId
              ? {
                  ...item,
                  environments: { ...item.environments, [environment]: config },
                }
              : item,
          ),
        },
      });
      onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };
  const saveResource = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!resource) return;
    setError("");
    try {
      const saved = await save.mutateAsync({
        ...data,
        config: {
          ...data.config,
          resources: [
            ...data.config.resources.filter((item) => item.id !== resource.id),
            resource,
          ],
        },
      });
      setData(saved);
      setConfig({
        ...config,
        nodes: [...new Set([...config.nodes, resource.id])],
      });
      setResource(undefined);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };
  const saveCertificate = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!certificateFile || !keyFile) return;
    setError("");
    if (certificateFile.size + keyFile.size > 100000) {
      setError(t(($) => $.application_deployment.file_size));
      return;
    }
    try {
      const saved = await upload.mutateAsync({
        name: certificateName,
        certificate: await certificateFile.text(),
        private_key: await keyFile.text(),
        domain: config.domain,
      });
      setConfig({ ...config, certificate_id: saved.id });
      setUploading(false);
      setCertificateFile(undefined);
      setKeyFile(undefined);
      setCertificateName("");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };
  const cancelNested = () => {
    setResource(undefined);
    setUploading(false);
    setCertificateFile(undefined);
    setKeyFile(undefined);
    setError("");
  };
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) {
          if (resource || uploading) cancelNested();
          else onClose();
        }
      }}
    >
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>
            {uploading
              ? t(($) => $.application_deployment.upload_certificate)
              : resource
                ? t(($) => $.application_deployment.add_server)
                : `${app.name} · ${t(($) => $.releases[environment])}`}
          </DialogTitle>
        </DialogHeader>
        {resource ? (
          <form className="space-y-4" onSubmit={saveResource}>
            <DeploymentResourceFields
              projectId={projectId}
              resource={resource}
              onChange={setResource}
            />
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={busy}
                onClick={cancelNested}
              >
                {t(($) => $.releases.cancel)}
              </Button>
              <Button
                type="submit"
                disabled={busy || !resource.credential_id || !resource.host_key}
              >
                {t(($) => $.application_deployment.save)}
              </Button>
            </div>
          </form>
        ) : uploading ? (
          <form className="space-y-4" onSubmit={saveCertificate}>
            <label className="block space-y-1">
              {t(($) => $.application_deployment.certificate_name)}
              <Input
                required
                value={certificateName}
                onChange={(e) => setCertificateName(e.target.value)}
              />
            </label>
            <label className="block space-y-1">
              {t(($) => $.application_deployment.pem_file)}
              <Input
                type="file"
                accept=".pem"
                required
                onChange={(e) => setCertificateFile(e.target.files?.[0])}
              />
              <span className="text-caption text-muted-foreground">
                {t(($) => $.application_deployment.pem_hint)}
              </span>
            </label>
            <label className="block space-y-1">
              {t(($) => $.application_deployment.key_file)}
              <Input
                type="file"
                accept=".key"
                required
                onChange={(e) => setKeyFile(e.target.files?.[0])}
              />
              <span className="text-caption text-muted-foreground">
                {t(($) => $.application_deployment.key_hint)}
              </span>
            </label>
            {!certificates.data?.encryption_configured && (
              <p role="alert">
                {t(($) => $.application_deployment.encryption_required)}
              </p>
            )}
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={busy}
                onClick={cancelNested}
              >
                {t(($) => $.releases.cancel)}
              </Button>
              <Button
                type="submit"
                disabled={busy || !certificates.data?.encryption_configured}
              >
                {t(($) => $.application_deployment.validate_save)}
              </Button>
            </div>
          </form>
        ) : (
          <>
            <ol className="grid grid-cols-2 gap-2 text-caption sm:grid-cols-4">
              {steps.map((label, index) => (
                <li
                  key={label}
                  aria-current={step === index ? "step" : undefined}
                  className={`rounded-lg p-2 ${step === index ? "bg-brand-subtle text-brand" : "bg-muted text-muted-foreground"}`}
                >
                  {index + 1} · {label}
                </li>
              ))}
            </ol>
            <form
              className="space-y-4"
              onSubmit={(event) => {
                event.preventDefault();
                setError("");
                if (step === 1 && !config.nodes.length) {
                  setError(t(($) => $.application_deployment.select_nodes));
                  return;
                }
                if (step < 3) setStep(step + 1);
                else void saveConfig();
              }}
            >
              {step === 0 && (
                <div className="grid gap-3 sm:grid-cols-2">
                  {(["nginx", "docker", "process", "kubernetes"] as const).map(
                    (kind) => (
                      <Button
                        type="button"
                        key={kind}
                        variant={config.kind === kind ? "brand" : "outline"}
                        onClick={() => setConfig({ ...config, kind })}
                      >
                        {t(($) => $.application_deployment[kind])}
                      </Button>
                    ),
                  )}
                </div>
              )}
              {step === 1 && (
                <>
                  <div className="space-y-2">
                    {data.config.resources
                      .filter((item) => item.env === environment)
                      .map((item) => (
                        <label
                          key={item.id}
                          className="flex items-center gap-3 rounded-lg border p-3"
                        >
                          <input
                            type="checkbox"
                            checked={config.nodes.includes(item.id)}
                            onChange={(e) =>
                              setConfig({
                                ...config,
                                nodes: e.target.checked
                                  ? [...config.nodes, item.id]
                                  : config.nodes.filter((id) => id !== item.id),
                              })
                            }
                          />
                          <span>
                            {item.name}
                            <span className="block text-caption text-muted-foreground">
                              {item.host}
                            </span>
                          </span>
                        </label>
                      ))}
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() =>
                      setResource({
                        id: crypto.randomUUID(),
                        name: "",
                        env: environment,
                        host: "",
                        port: "22",
                        username: "deploy",
                        host_key: "",
                        credential_id: "",
                      })
                    }
                  >
                    {t(($) => $.application_deployment.add_server)}
                  </Button>
                </>
              )}
              {step === 2 && (
                <>
                  <div className="grid gap-4 sm:grid-cols-2">
                    {input("directory", true)}
                    {config.kind === "docker" && input("port", true)}
                    {(config.kind === "process" ||
                      config.kind === "kubernetes") &&
                      input("command", true)}
                    {config.kind === "kubernetes" && input("namespace", true)}
                    {input("domain", true)}
                    <ConfigurationSelect
                      label="HTTPS"
                      value={config.https}
                      onChange={(value) =>
                        setConfig({
                          ...config,
                          https: value as ApplicationDeployment["https"],
                        })
                      }
                      items={[
                        {
                          value: "off",
                          label: t(($) => $.application_deployment.http),
                        },
                        {
                          value: "existing",
                          label: t(
                            ($) =>
                              $.application_deployment.existing_certificate,
                          ),
                        },
                        {
                          value: "managed",
                          label: t(
                            ($) => $.application_deployment.managed_certificate,
                          ),
                        },
                      ]}
                    />
                  </div>
                  {config.https === "existing" && (
                    <div className="space-y-3 rounded-lg border p-3">
                      <ConfigurationSelect
                        label={t(($) => $.application_deployment.certificate)}
                        value={config.certificate_id}
                        onChange={(value) =>
                          setConfig({ ...config, certificate_id: value })
                        }
                        items={[
                          {
                            value: "",
                            label: t(($) => $.application_deployment.choose),
                          },
                          ...(certificates.data?.certificates ?? []).map(
                            (item) => ({ value: item.id, label: item.name }),
                          ),
                        ]}
                      />
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => setUploading(true)}
                      >
                        {t(($) => $.application_deployment.upload_certificate)}
                      </Button>
                      {certificate && (
                        <p className="text-caption">
                          {certificate.domains.join(", ")} ·{" "}
                          {t(($) => $.application_deployment.expires)}{" "}
                          {new Date(certificate.not_after).toLocaleDateString()}
                        </p>
                      )}
                      {certificates.isError && (
                        <p role="alert">{certificates.error.message}</p>
                      )}
                    </div>
                  )}
                  {config.https === "managed" && (
                    <p className="text-caption text-muted-foreground">
                      {t(($) => $.application_deployment.managed_hint)}
                    </p>
                  )}
                  {config.nodes.length > 1 && (
                    <ConfigurationSelect
                      label={t(($) => $.application_deployment.strategy)}
                      value={config.strategy}
                      onChange={(value) =>
                        setConfig({
                          ...config,
                          strategy: value as ApplicationDeployment["strategy"],
                        })
                      }
                      items={(["serial", "rolling", "batch"] as const).map(
                        (value) => ({
                          value,
                          label: t(($) => $.application_deployment[value]),
                        }),
                      )}
                    />
                  )}
                  <details>
                    <summary>
                      {t(($) => $.application_deployment.advanced)}
                    </summary>
                    <div className="mt-3 space-y-3">
                      {(config.kind === "nginx" || config.kind === "docker") && (
                        <label className="block space-y-1">
                          {t(($) => $.application_deployment.nginx_config_directory)}
                          <Input
                            value={config.nginx_config_directory ?? "/etc/nginx/conf.d"}
                            onChange={(event) =>
                              setConfig({ ...config, nginx_config_directory: event.target.value })
                            }
                          />
                        </label>
                      )}
                      {input("health_path")}
                      {config.kind === "nginx" && (
                        <label className="flex items-center gap-2">
                          <input
                            type="checkbox"
                            checked={config.spa}
                            onChange={(e) =>
                              setConfig({ ...config, spa: e.target.checked })
                            }
                          />
                          {t(($) => $.application_deployment.spa)}
                        </label>
                      )}
                    </div>
                  </details>
                </>
              )}
              {step === 3 && (
                <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-3 text-label">
                  <dt>{t(($) => $.application_deployment.method)}</dt>
                  <dd>{t(($) => $.application_deployment[config.kind])}</dd>
                  <dt>{t(($) => $.application_deployment.location)}</dt>
                  <dd>
                    {data.config.resources
                      .filter((item) => config.nodes.includes(item.id))
                      .map((item) => item.name)
                      .join(", ")}
                  </dd>
                  <dt>{t(($) => $.application_deployment.domain)}</dt>
                  <dd>{config.domain}</dd>
                  <dt>HTTPS</dt>
                  <dd>
                    {config.https === "existing"
                      ? certificate?.name ||
                        t(($) => $.application_deployment.choose)
                      : config.https}
                  </dd>
                </dl>
              )}
              <div className="flex flex-wrap justify-between gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  disabled={busy}
                  onClick={() => void saveConfig()}
                >
                  {t(($) => $.application_deployment.save_draft)}
                </Button>
                <div className="flex gap-2">
                  {step > 0 && (
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => setStep(step - 1)}
                    >
                      {t(($) => $.application_deployment.previous)}
                    </Button>
                  )}
                  <Button
                    type="submit"
                    disabled={
                      busy ||
                      (step >= 2 &&
                        config.https === "existing" &&
                        !config.certificate_id)
                    }
                  >
                    {step < 3
                      ? t(($) => $.application_deployment.next)
                      : t(($) => $.application_deployment.save_return)}
                  </Button>
                </div>
              </div>
            </form>
          </>
        )}
        {error && (
          <p role="alert" className="text-caption text-destructive">
            {error}
          </p>
        )}
      </DialogContent>
    </Dialog>
  );
}
