import { z } from "zod";

// Error messages are translation keys (namespace "validation"); components translate them.
const dob = z.string().min(1, "required").refine((v) => !Number.isNaN(Date.parse(v)) && new Date(v) < new Date(), "pastDate");
const optionalDob = z.string().optional().refine((v) => !v || (!Number.isNaN(Date.parse(v)) && new Date(v) < new Date()), "pastDate");

export const maritalStatuses = ["MARRIED", "SINGLE_FEMALE", "SINGLE_MALE", "DIVORCED", "WIDOWED"] as const;
export const ageGroups = ["0-2", "2-4", "4-8", "8-18"] as const;

export const eligibilitySchema = z
  .object({
    marital_status: z.enum(maritalStatuses),
    applicant_dob: dob,
    spouse_dob: optionalDob,
    years_married: z.number({ error: "number" }).min(0, "min0").max(80, "max80").optional(),
    existing_children: z.number({ error: "number" }).int("integer").min(0, "min0").max(30, "max30"),
  })
  .superRefine((v, ctx) => {
    if (v.marital_status === "MARRIED") {
      if (!v.spouse_dob) ctx.addIssue({ code: "custom", path: ["spouse_dob"], message: "required" });
      if (v.years_married === undefined || Number.isNaN(v.years_married)) ctx.addIssue({ code: "custom", path: ["years_married"], message: "required" });
    }
  });
export type EligibilityForm = z.infer<typeof eligibilitySchema>;

export const loginSchema = z.object({
  email: z.string().min(1, "required").email("email"),
  password: z.string().min(1, "required"),
  otp: z.string().optional(),
});
export type LoginForm = z.infer<typeof loginSchema>;

export const registerSchema = z.object({
  full_name: z.string().min(2, "required"),
  email: z.string().min(1, "required").email("email"),
  phone: z.string().regex(/^[+]?[0-9 -]{10,15}$/, "phone"),
  password: z.string().min(8, "password8"),
  language: z.enum(["en", "hi", "mr"]),
  consent: z.literal(true, { error: "consent" }),
});
export type RegisterForm = z.infer<typeof registerSchema>;

export const profileSchema = z
  .object({
    marital_status: z.enum(maritalStatuses),
    years_married: z.number({ error: "number" }).min(0).max(80).optional(),
    applicant_dob: dob,
    spouse_dob: optionalDob,
    existing_children: z.number({ error: "number" }).int("integer").min(0, "min0").max(30, "max30"),
    annual_income_inr: z.number({ error: "number" }).min(0, "min0").optional(),
    state: z.string().min(1, "required"),
    district: z.string().min(1, "required"),
    address: z.string().optional(),
    pref_age_group: z.enum(ageGroups),
    pref_gender: z.enum(["ANY", "MALE", "FEMALE"]),
    pref_states: z.array(z.string()),
    open_special_needs: z.boolean(),
    open_siblings: z.boolean(),
  })
  .superRefine((v, ctx) => {
    if (v.marital_status === "MARRIED" && !v.spouse_dob) ctx.addIssue({ code: "custom", path: ["spouse_dob"], message: "required" });
  });
export type ProfileForm = z.infer<typeof profileSchema>;

export const scamSchema = z.object({ text: z.string().trim().min(10, "min10").max(4000, "max4000") });
export const messageSchema = z.object({ body: z.string().trim().min(1, "required").max(4000, "max4000") });
export const assistantSchema = z.object({ question: z.string().trim().min(2, "required").max(1000, "max1000") });

export const hsrNotesSchema = z.object({
  visit_date: z.string().min(1, "required").refine((v) => new Date(v) <= new Date(), "pastDate"),
  raw_notes: z.string().trim().min(20, "min20").max(12000, "max12000"),
});

export const addChildSchema = z.object({
  name: z.string().trim().min(2, "required"),
  dob: dob,
  gender: z.enum(["MALE", "FEMALE"]),
  is_special_needs: z.boolean(),
  admitted_on: z.string().min(1, "required"),
  cwc_lfa_number: z.string().min(1, "required"),
  lfa_date: z.string().min(1, "required"),
});
export type AddChildForm = z.infer<typeof addChildSchema>;

export const legalEventSchema = z.object({
  event_type: z.enum(["MATCHED", "FOSTER_CARE", "DM_FILED", "DM_ORDER", "BIRTH_CERT"]),
  event_date: z.string().min(1, "required"),
  reference_no: z.string().optional(),
});
export type LegalEventForm = z.infer<typeof legalEventSchema>;

export const knowledgeSchema = z.object({
  source_title: z.string().min(3, "required"),
  source_url: z.string().url("url").optional().or(z.literal("")),
  language: z.enum(["en", "hi", "mr"]),
  text: z.string().min(100, "min100"),
});

export const reuploadNoteSchema = z.object({ note: z.string().trim().min(5, "min5").max(500, "max500") });
export const notReserveSchema = z.object({ reason: z.string().trim().min(5, "min5").max(1000, "max1000") });

/** Upload rules from guide 8.1: PDF, JPG, PNG up to 5 MB. Server re-checks the real file type. */
export const ALLOWED_DOC_TYPES = ["application/pdf", "image/jpeg", "image/png"] as const;
export const MAX_DOC_BYTES = 5 * 1024 * 1024;
export function validateDocFile(file: File): "type" | "size" | "empty" | null {
  if (file.size === 0) return "empty";
  if (!(ALLOWED_DOC_TYPES as readonly string[]).includes(file.type)) return "type";
  if (file.size > MAX_DOC_BYTES) return "size";
  return null;
}
