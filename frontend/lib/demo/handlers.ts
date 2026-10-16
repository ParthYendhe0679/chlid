/**
 * EXPLICIT DEMO MODE ONLY (NEXT_PUBLIC_DEMO_MODE=true). Synthetic data, in memory, no persistence.
 * Never used as a fallback when the real API fails. Results are NOT real AI analysis or legal information.
 * Test fixtures live separately in tests/.
 */
import { ApiError } from "@/lib/api/errors";
import type { SessionUser } from "@/types";

type Q = Record<string, string | number | boolean | undefined | null> | undefined;

const USERS: Record<string, SessionUser> = {
  "parent@demo.in": { id: "u1", role: "PARENT", full_name: "Demo Parent One", language: "en" },
  "parent2@demo.in": { id: "u2", role: "PARENT", full_name: "Demo Parent Two", language: "en" },
  "worker@demo.in": { id: "u3", role: "AGENCY", agency_id: "a1", full_name: "Demo Social Worker", language: "en", designation: "SOCIAL_WORKER" },
  "doctor@demo.in": { id: "u4", role: "AGENCY", agency_id: "a1", full_name: "Demo Medical Officer", language: "en", designation: "MEDICAL_OFFICER" },
  "admin@demo.in": { id: "u5", role: "ADMIN", full_name: "Demo Oversight Officer", language: "en" },
};

let session: SessionUser | null = null;
const day = 86_400_000;
const iso = (offsetDays: number) => new Date(Date.now() + offsetDays * day).toISOString();
const delay = (ms = 350) => new Promise((r) => setTimeout(r, ms));
const err = (status: number, code: ConstructorParameters<typeof ApiError>[1], msg: string) => new ApiError(status, code, msg, "demo");

const CATS = ["PHOTO", "IDENTITY", "BIRTH_PROOF", "RESIDENCE", "INCOME", "MEDICAL", "MARRIAGE", "REFERENCE"] as const;
const docs: Record<string, { status: string; id: string }> = {};
let appStatus = "REGISTERED";
let referralDecided = false;
let hsrSigned = false;
const messages = [{ id: "m1", sender_name: "Demo Social Worker", sender_role: "AGENCY", body: "Welcome. Please upload your documents.", created_at: iso(-1) }];

function requireRole(role: string) {
  if (!session) throw err(401, "UNAUTHENTICATED", "Please log in.");
  if (session.role !== role) throw err(403, "FORBIDDEN", "You cannot do this.");
}

function eligibility(b: { marital_status: string; applicant_dob: string; spouse_dob?: string; years_married?: number; existing_children: number }) {
  const age = (d: string) => Math.floor((Date.now() - new Date(d).getTime()) / (365.25 * day));
  const married = b.marital_status === "MARRIED";
  const reasons: { code: string; message: string }[] = [];
  const limits: [string, number, number][] = [["0-2", 85, 40], ["2-4", 90, 45], ["4-8", 100, 50], ["8-18", 110, 55]];
  const a = age(b.applicant_dob);
  const comp = married ? a + age(b.spouse_dob ?? b.applicant_dob) : a;
  let eligible = true;
  if (married && (b.years_married ?? 0) < 2) { eligible = false; reasons.push({ code: "MARRIAGE_DURATION", message: "A couple needs a stable marriage of at least 2 years." }); }
  if (b.existing_children >= 3) { eligible = false; reasons.push({ code: "CHILDREN_COUNT", message: "A couple with 3 or more children cannot adopt, with a few exceptions." }); }
  const allowed = eligible ? limits.filter(([, c, s]) => (married ? comp <= c : a <= s)).map(([g]) => g) : [];
  for (const [g, c, s] of limits) if (eligible && !allowed.includes(g)) reasons.push({ code: "AGE_" + g, message: `Age limit for children ${g} years is ${married ? c : s}; yours is ${comp}.` });
  if (allowed.length === 0) eligible = false;
  return { eligible, allowed_age_groups: allowed, reasons };
}

const radarItems = [
  { case_ref: "APP-0193", agency: "Asha Shishu Griha", step: "HSR_60D", due_at: iso(4), days_left: 4, risk_score: 0.78, risk_reasons: ["80% of allowed time used", "Agency has 3 times usual workload", "Documents re-uploaded twice"], level: "HIGH_RISK" },
  { case_ref: "APP-0077", agency: "Nashik Bal Seva", step: "DM_ORDER_60D", due_at: iso(-6), days_left: -6, risk_score: 0.93, risk_reasons: ["Deadline already passed"], level: "OVERDUE" },
  { case_ref: "APP-0210", agency: "Pune Shishu Kendra", step: "MATCHING_30D", due_at: iso(21), days_left: 21, risk_score: 0.22, risk_reasons: [], level: "ON_TRACK" },
];

export async function demoRequest(method: string, path: string, body: unknown, query?: Q): Promise<unknown> {
  await delay();
  const b = (body ?? {}) as Record<string, unknown>;

  if (path === "/auth/login") {
    const u = USERS[String(b.email).toLowerCase()];
    if (!u || b.password !== "Demo@1234") throw err(401, "UNAUTHENTICATED", "Invalid credentials");
    session = u;
    return { user: u };
  }
  if (path === "/auth/register") { session = { id: "u9", role: "PARENT", full_name: String(b.full_name), language: (b.language as SessionUser["language"]) ?? "en" }; return { user: session }; }
  if (path === "/auth/logout") { session = null; return { ok: true }; }
  if (path === "/auth/refresh") { if (!session) throw err(401, "UNAUTHENTICATED", "No session"); return {}; }
  if (path === "/auth/me") { if (!session) throw err(401, "UNAUTHENTICATED", "Not logged in"); return { user: session }; }

  // public
  if (path === "/public/eligibility") return eligibility(b as never);
  if (path === "/public/fees") return { items: [{ name: "Home Study Report", amount: 6000 }, { name: "Adoption fee", amount: 50000 }, { name: "Follow-up visits (4 x 2000)", amount: 8000 }], total_single_child: 64000, notes: ["Fees are non-refundable.", "No extra payment is required."] };
  if (path === "/public/agencies") {
    const all = [{ name: "Asha Shishu Griha", reg_number: "MH-SAA-0123", type: "SAA", district: "Thane", state: "Maharashtra", verified: true }, { name: "Pune Shishu Kendra", reg_number: "MH-SAA-0456", type: "SAA", district: "Pune", state: "Maharashtra", verified: true }];
    const q = String(query?.q ?? "").toLowerCase();
    return { agencies: all.filter((a) => a.name.toLowerCase().includes(q)) };
  }
  if (path === "/public/scam-check") {
    const text = String(b.text).toLowerCase();
    const bad = /advance|no paperwork|whatsapp|baby available/.test(text);
    return { verdict: bad ? "LIKELY_SCAM" : "LOOKS_SAFE", reasons: bad ? ["Asks for advance payment", "Offers a child without the legal process"] : ["No known warning phrases found"], advice: "In India, legal adoption happens only through CARA, the CARINGS website and registered agencies. Buying or selling a child is a crime. Report it to Childline 1098 or the police.", report_token: "rt_demo" };
  }
  if (path === "/public/scam-report") return { ok: true };

  // parent
  if (path === "/applications/me" && method === "GET") {
    requireRole("PARENT");
    return {
      id: "app1", status: appStatus, doc_deadline: iso(12), marital_status: "MARRIED", applicant_dob: "1990-01-01", spouse_dob: "1991-01-01", years_married: 5, existing_children: 0, state: "Maharashtra", district: "Thane",
      pref_age_group: "0-2", pref_gender: "ANY", pref_states: [], open_special_needs: false, open_siblings: false, referrals_used: 1, seniority_date: appStatus === "REGISTERED" ? null : iso(-30),
      required_docs: CATS.map((c) => ({ category: c, uploaded: !!docs[c], status: docs[c]?.status ?? null, document_id: docs[c]?.id })),
    };
  }
  if (path === "/applications/me" && method === "PATCH") return {};
  if (path === "/applications") return { application_id: "app1", status: "REGISTERED", doc_deadline: iso(30) };
  if (path === "/applications/me/tracker") {
    requireRole("PARENT");
    const names = ["Registered", "Documents submitted", "Home study", "Waiting list", "Referral", "Matching", "Adoption order", "Follow-up"];
    const idx = appStatus === "REGISTERED" ? 0 : appStatus === "DOCS_SUBMITTED" ? 1 : 3;
    return { status: appStatus, stage_index: idx, stages: names.map((title, i) => ({ key: String(i), title, done: i < idx, date: i < idx ? iso(-40 + i * 5) : null })), current_deadline: { step: appStatus === "REGISTERED" ? "DOC_UPLOAD_30D" : "HSR_60D", due_at: iso(12), days_left: 12 }, next_step_text: appStatus === "REGISTERED" ? "Upload your documents. DocCheck AI will check each one." : "Your social worker will review your documents.", referrals_used: 1 };
  }
  if (path === "/applications/me/documents") {
    const form = body as FormData;
    const file = form.get("file") as File;
    const cat = String(form.get("category"));
    const rent = /rent/i.test(file.name);
    const status = rent ? "AI_WARN" : "AI_PASS";
    docs[cat] = { status, id: "d-" + cat };
    await delay(1800);
    return { document_id: "d-" + cat, status, detected_type: rent ? "RENT_AGREEMENT" : "ELECTRICITY_BILL", confidence: 0.91, checks: [
      rent ? { code: "RESIDENCE_TYPE_NOT_IN_LIST", result: "WARN", message: "A rent agreement is not in the usual list of accepted residence proofs. Please upload an accepted document, or your social worker will review this one." } : { code: "TYPE_ALLOWED", result: "PASS", message: "Document type is accepted for this category." },
      { code: "NAME_MATCH", result: "PASS", message: "The name matches your application." }] };
  }
  if (path === "/applications/me/submit") { appStatus = "DOCS_SUBMITTED"; return { status: "DOCS_SUBMITTED", seniority_date: iso(0) }; }
  if (path.startsWith("/documents/")) return { url: "about:blank", expires_in: 300 };
  if (path === "/referrals/active") {
    requireRole("PARENT");
    if (session?.id !== "u2" || referralDecided) return null;
    return { referral_id: "r1", referral_number: 2, expires_at: iso(1.5), child: { case_code: "MH-THN-2026-0042", age_text: "1 year 4 months", gender: "FEMALE", is_special_needs: false, summary_approved: "SYNTHETIC DEMO TEXT. The medical report shows a healthy child with normal growth. Ask a doctor if you have questions.", csr_url: "about:blank", mer_url: "about:blank" } };
  }
  if (path.startsWith("/referrals/") && path.endsWith("/decision")) { referralDecided = true; return { status: b.decision === "RESERVED" ? "RESERVED" : "WAITING", warning: b.decision === "RESERVED" ? undefined : "You have used 2 of 3 referrals." }; }
  if (path === "/followups/me") return [];
  if (/^\/applications\/[^/]+\/messages$/.test(path)) {
    if (method === "POST") { const m = { id: "m" + (messages.length + 1), sender_name: session?.full_name ?? "", sender_role: session?.role ?? "PARENT", body: String(b.body), created_at: iso(0) }; messages.push(m as never); return m; }
    return messages;
  }
  if (path === "/ai/assistant") {
    const q = String(b.question).toLowerCase();
    const own = /my case|my report|why is my/.test(q);
    return own
      ? { answer: "I cannot see your case. Your social worker can answer this.", citations: [], grounded: false, is_about_own_case: true, offer_escalation: true }
      : { answer: "If you do not reserve a child in 3 referrals, you cannot apply again for 1 year. (Demo answer.)", citations: [{ source_title: "Adoption Regulations 2022", citation: "Regulation 9(3)", source_url: "https://cara.wcd.gov.in/" }], grounded: true, is_about_own_case: false, offer_escalation: false };
  }
  if (path === "/ai/assistant/escalate") return { message_id: "m99" };

  // agency
  if (path === "/agency/cases") { requireRole("AGENCY"); return [{ application_id: "app1", parent_name: "Rahul and Priya Sharma", status: "DOCS_SUBMITTED", current_deadline: { step: "HSR_60D", due_at: iso(4), days_left: 4, risk_score: 0.78, risk_reasons: ["80% of allowed time used"] } }, { application_id: "app2", parent_name: "Demo Parent Two", status: "WAITING" }]; }
  if (/^\/agency\/cases\/[^/]+$/.test(path)) {
    return { application_id: "app1", parent_name: "Rahul and Priya Sharma", status: hsrSigned ? "WAITING" : "HSR_IN_PROGRESS", current_deadline: { step: "HSR_60D", due_at: iso(4), days_left: 4, risk_score: 0.78, risk_reasons: ["80% of allowed time used", "Documents re-uploaded twice"] },
      documents: [{ id: "d1", category: "RESIDENCE", file_name: "rent_agreement.pdf", status: "AI_WARN", ai_detected_type: "RENT_AGREEMENT", ai_confidence: 0.91, ai_fields: { name: "Rahul Sharma", address: "B-12, Shanti Nagar, Mira Road", id_number_last4: "4417" }, ai_checks: [{ code: "RESIDENCE_TYPE_NOT_IN_LIST", result: "WARN", message: "A rent agreement is not in the usual list of accepted residence proofs." }, { code: "NAME_MATCH", result: "PASS", message: "The name matches." }], uploaded_at: iso(-3) }],
      hsr: hsrSigned ? { final_sections: {}, missing_items: [], completeness_score: 100, signed_at: iso(0) } : null, timeline: [], followups: [] };
  }
  if (path.startsWith("/agency/documents/")) return { document: {}, application_status: "DOCS_SUBMITTED" };
  if (path === "/ai/hsr/draft") { await delay(1500); return { sections: { family_background: "Mr Rahul Sharma (36) and Mrs Priya Sharma (35) live in Mira Road. (Demo draft from notes.)", home_environment: "Two-bedroom flat in a quiet neighbourhood.", health: "Both report good health.", finances: "Stable salaried income.", motivation_readiness: "Long-standing wish to adopt.", parenting_plan: "Mother plans leave for the first year.", support_system: "", observations: "Warm, cooperative couple." }, missing_items: ["support_system"], completeness_score: 88, ai_run_id: "air_51" }; }
  if (path.startsWith("/agency/hsr/") && path.endsWith("/sign")) { hsrSigned = true; return { status: "WAITING", hsr_valid_until: iso(365 * 3) }; }
  if (path.startsWith("/agency/hsr/")) return { ok: true };

  // admin
  if (path === "/admin/radar") {
    requireRole("ADMIN");
    const items = radarItems.filter((i) => (!query?.level || i.level === query.level) && (!query?.step || i.step === query.step));
    return { summary: { overdue: 12, high_risk: 9, on_track: 140 }, items };
  }
  if (path === "/admin/agency-scorecard") return [{ agency: "Asha Shishu Griha", avg_days: { HSR_60D: 48, DM_ORDER_60D: 71 }, late_rate: 0.22, open_cases: 34 }, { agency: "Pune Shishu Kendra", avg_days: { HSR_60D: 39, DM_ORDER_60D: 55 }, late_rate: 0.1, open_cases: 21 }];
  if (path === "/admin/radar/recompute") return { updated: 151 };
  if (path === "/admin/child-alerts") return [{ case_code: "MH-NSK-2025-0011", agency: "Nashik Bal Seva", reason: "NO_FAMILY_VISIT_12M", days: 430 }, { case_code: "MH-KOP-2026-0003", agency: "Kolhapur Bal Griha", reason: "REPORTS_MISSING_10D", days: 14 }];
  if (path === "/admin/agencies") return [{ id: "a9", name: "Demo Pending Agency", reg_number: "MH-SAA-0999", agency_type: "SAA", state: "Maharashtra", district: "Nagpur", is_approved: false }];
  if (path.startsWith("/admin/agencies/")) return {};
  if (path === "/admin/knowledge") return { chunks_created: 42 };
  if (path === "/admin/scam-reports") return [{ id: "s1", text: "Healthy newborn baby boy available for adoption, no paperwork needed, Rs 50,000 advance.", verdict: "LIKELY_SCAM", reasons: ["Asks for advance payment"], reviewed: false, created_at: iso(-1) }];
  if (path === "/admin/audit/verify") return { valid: true, rows_checked: 5231 };
  if (path === "/admin/audit") return [{ id: 2, actor_role: "AGENCY", actor_id: "u3-demo", action: "VERIFY_DOC", target_type: "document", target_id: "d1", created_at: iso(-1) }, { id: 1, actor_role: "ADMIN", actor_id: "u5-demo", action: "LOGIN", target_type: "user", target_id: "u5", created_at: iso(-2) }];

  throw err(404, "NOT_FOUND", `Demo mode has no handler for ${method} ${path}`);
}
