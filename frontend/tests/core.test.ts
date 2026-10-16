import { describe, expect, it, vi, beforeEach } from "vitest";
import fs from "node:fs";
import path from "node:path";
import en from "@/messages/en.json";
import { parseApiError, ApiError } from "@/lib/api/errors";
import { validateDocFile, eligibilitySchema, registerSchema, MAX_DOC_BYTES } from "@/lib/validation/schemas";
import { requiredRoleFor } from "@/lib/permissions";

describe("api error mapping", () => {
  it("uses the server error code and request id", () => {
    const e = parseApiError(403, { error: { code: "FORBIDDEN", message: "No.", request_id: "r1" } });
    expect(e).toBeInstanceOf(ApiError);
    expect(e.code).toBe("FORBIDDEN");
    expect(e.requestId).toBe("r1");
    expect(e.isForbidden).toBe(true);
  });
  it("falls back to status when the body is not the contract shape", () => {
    expect(parseApiError(401, null).code).toBe("UNAUTHENTICATED");
    expect(parseApiError(409, "x").code).toBe("INVALID_TRANSITION");
    expect(parseApiError(503, undefined).code).toBe("AI_UNAVAILABLE");
    expect(parseApiError(500, {}).code).toBe("UNKNOWN");
  });
});

describe("document file validation", () => {
  const f = (type: string, size: number) => new File([new Uint8Array(size)], "a", { type });
  it("accepts pdf/jpg/png within 5 MB", () => {
    expect(validateDocFile(f("application/pdf", 10))).toBeNull();
    expect(validateDocFile(f("image/png", 10))).toBeNull();
  });
  it("rejects wrong type, oversize and empty files", () => {
    expect(validateDocFile(f("text/plain", 10))).toBe("type");
    expect(validateDocFile(f("image/jpeg", MAX_DOC_BYTES + 1))).toBe("size");
    expect(validateDocFile(f("image/jpeg", 0))).toBe("empty");
  });
});

describe("schemas", () => {
  it("requires spouse details for married applicants", () => {
    const r = eligibilitySchema.safeParse({ marital_status: "MARRIED", applicant_dob: "1990-01-01", existing_children: 0 });
    expect(r.success).toBe(false);
  });
  it("accepts a valid single applicant", () => {
    expect(eligibilitySchema.safeParse({ marital_status: "SINGLE_FEMALE", applicant_dob: "1990-01-01", existing_children: 0 }).success).toBe(true);
  });
  it("requires consent and a strong enough password to register", () => {
    const base = { full_name: "A B", email: "a@b.in", phone: "9999999999", password: "Demo@1234", language: "en" as const };
    expect(registerSchema.safeParse({ ...base, consent: false }).success).toBe(false);
    expect(registerSchema.safeParse({ ...base, consent: true }).success).toBe(true);
    expect(registerSchema.safeParse({ ...base, password: "short", consent: true }).success).toBe(false);
  });
});

describe("role routing", () => {
  it("maps paths to the only role allowed", () => {
    expect(requiredRoleFor("/parent/documents")).toBe("PARENT");
    expect(requiredRoleFor("/agency/cases/1")).toBe("AGENCY");
    expect(requiredRoleFor("/admin")).toBe("ADMIN");
    expect(requiredRoleFor("/eligibility")).toBeNull();
  });
});

describe("api client", () => {
  beforeEach(() => { vi.resetModules(); vi.restoreAllMocks(); });
  it("sends credentials and maps an error response", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ error: { code: "NOT_FOUND", message: "gone", request_id: "x" } }), { status: 404 }),
    );
    const { api } = await import("@/lib/api/client");
    await expect(api("/applications/me")).rejects.toMatchObject({ code: "NOT_FOUND" });
    expect(spy.mock.calls[0][1]).toMatchObject({ credentials: "include", method: "GET" });
  });
  it("does not retry a failed POST", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("net"));
    const { api } = await import("@/lib/api/client");
    await expect(api("/referrals/1/decision", { method: "POST", body: {} })).rejects.toMatchObject({ code: "NETWORK_ERROR" });
    expect(spy).toHaveBeenCalledTimes(1);
  });
  it("retries an idempotent GET once on network failure", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("net"));
    const { api } = await import("@/lib/api/client");
    await expect(api("/public/fees")).rejects.toMatchObject({ code: "NETWORK_ERROR" });
    expect(spy).toHaveBeenCalledTimes(2);
  });
});

// Guards against raw translation keys appearing in the UI: every literal t("ns.key") must exist in en.json.
describe("translation key coverage", () => {
  const root = path.resolve(__dirname, "..");
  const files: string[] = [];
  const walk = (dir: string) => {
    for (const n of fs.readdirSync(dir, { withFileTypes: true })) {
      if (["node_modules", ".next", "tests"].includes(n.name)) continue;
      const p = path.join(dir, n.name);
      if (n.isDirectory()) walk(p);
      else if (/\.tsx?$/.test(n.name)) files.push(p);
    }
  };
  walk(root);
  const get = (obj: unknown, key: string) => key.split(".").reduce<unknown>((o, k) => (o && typeof o === "object" ? (o as Record<string, unknown>)[k] : undefined), obj);

  it("has every statically referenced key", () => {
    const missing: string[] = [];
    for (const f of files) {
      const src = fs.readFileSync(f, "utf8");
      // A function name can be reused for different namespaces in one file, so accept a key found under any of them.
      const byFn: Record<string, (string | null)[]> = {};
      for (const m of src.matchAll(/const (\w+) = (?:await )?(?:useTranslations|getTranslations)\((?:"([\w.]+)")?\)/g)) (byFn[m[1]] ??= []).push(m[2] ?? null);
      for (const [fn, nss] of Object.entries(byFn)) {
        for (const m of src.matchAll(new RegExp(`\\b${fn}(?:\\.has)?\\(\\s*"([\\w.]+)"`, "g"))) {
          if (!nss.some((ns) => get(en, ns ? `${ns}.${m[1]}` : m[1]) !== undefined)) missing.push(`${path.relative(root, f)}: ${m[1]}`);
        }
      }
    }
    expect(missing).toEqual([]);
  });
});
