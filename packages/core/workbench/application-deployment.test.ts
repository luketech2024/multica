// @vitest-environment node
import { describe, it, expect } from "vitest";
import {
  ProjectDeploymentSchema,
  DeploymentCertificateSchema,
  ApplicationReleaseSchema,
  DeploymentConnectionCheckSchema,
  DeploymentHostDiscoverySchema,
  DeploymentResourceSchema,
} from "./application-deployment-schemas";
import { ReleaseOrderSchema } from "./release-schemas";

describe("application deployment API contracts", () => {
  it("defaults additive configuration fields and rejects malformed identities", () => {
    expect(DeploymentHostDiscoverySchema.safeParse({host_key:"invalid"}).success).toBe(false);
    expect(DeploymentHostDiscoverySchema.parse({host_key:"SHA256:"+"A".repeat(43)}).host_key).toMatch(/^SHA256:/);
    const server = {id:"019a6091-9363-7000-8000-000000000001",name:"Server",env:"staging",host:"127.0.0.1"};
    expect(DeploymentResourceSchema.parse(server).auth_method).toBeUndefined();
    expect(DeploymentResourceSchema.parse({...server,auth_method:"password"}).auth_method).toBe("password");
    expect(DeploymentResourceSchema.safeParse({...server,auth_method:"unknown"}).success).toBe(false);
    expect(
      DeploymentConnectionCheckSchema.safeParse({ ok: "yes", detail: "" })
        .success,
    ).toBe(false);
    expect(
      ProjectDeploymentSchema.safeParse({
        config: {},
        revision: 1,
        deployments: [{ application_id: "invalid" }],
      }).success,
    ).toBe(false);
    expect(
      ProjectDeploymentSchema.parse({ config: {}, revision: 0 }).config,
    ).toEqual({ mode: "auto", applications: [], resources: [] });
    expect(
      ProjectDeploymentSchema.safeParse({
        config: { applications: [{ id: "invalid" }] },
        revision: 1,
      }).success,
    ).toBe(false);
    expect(
      ProjectDeploymentSchema.safeParse({ config: {}, revision: -1 }).success,
    ).toBe(false);
    expect(
      DeploymentCertificateSchema.safeParse({
        id: "invalid",
        name: "certificate",
        domains: [],
      }).success,
    ).toBe(false);
  });
  it("keeps application release data and rejects unknown change types", () => {
    const release = { title: "Website", mode: "auto", changes: [] };
    expect(ApplicationReleaseSchema.parse(release).results).toEqual([]);
    expect(
      ApplicationReleaseSchema.safeParse({
        ...release,
        changes: [{ type: "unknown" }],
      }).success,
    ).toBe(false);
    expect(
      ReleaseOrderSchema.shape.manifest.parse({
        tasks: [],
        repositories: [],
        delivery: release,
      }).delivery?.title,
    ).toBe("Website");
    expect(
      ReleaseOrderSchema.shape.manifest.parse({ tasks: [], repositories: [] })
        .delivery,
    ).toBeUndefined();
  });
});
