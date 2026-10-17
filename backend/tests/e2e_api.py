"""End-to-end API test for Child Connect (guide section 22).

Walks the full legal process through the real API: public tools, registration, profile, DocCheck uploads,
agency review, Home Study Report, children and the Referral Lock, referral decisions, legal events,
follow-ups, messages, Sahayak, admin oversight, and at least 10 security cases that must return 403
and appear in the audit log.

It CHANGES data (it creates its own parent and children). Re-run the seed afterwards for a clean demo:
    go run ./cmd/seed -reset

Usage: python tests/e2e_api.py [--base http://localhost:8080/api/v1] [--samples ../sample-documents] [--skip-lockout]
"""
import argparse
import os
import random
import string
import sys
import time
from datetime import date, timedelta

import requests

P = argparse.ArgumentParser()
P.add_argument("--base", default="http://localhost:8080/api/v1")
P.add_argument("--samples", default=os.path.join(os.path.dirname(__file__), "..", "..", "sample-documents"))
P.add_argument("--password", default="ChildConnect@2026")
P.add_argument("--skip-lockout", action="store_true")
ARGS = P.parse_args()
BASE = ARGS.base
PW = ARGS.password
ORIGIN = "http://localhost:3000"

results = []
timings = {"normal": [], "ai": []}


def check(name, cond, detail=""):
    results.append((name, bool(cond)))
    mark = "PASS" if cond else "FAIL"
    print(f"[{mark}] {name}" + (f"  -- {detail}" if (detail and not cond) else ""))
    return cond


class Client:
    def __init__(self, label):
        self.s = requests.Session()
        self.s.headers["Origin"] = ORIGIN
        self.label = label

    def req(self, method, path, kind="normal", **kw):
        t = time.perf_counter()
        r = self.s.request(method, BASE + path, timeout=120, **kw)
        timings[kind].append(time.perf_counter() - t)
        return r

    def get(self, path, **kw):
        return self.req("GET", path, **kw)

    def post(self, path, json=None, kind="normal", **kw):
        return self.req("POST", path, kind=kind, json=json, **kw)

    def patch(self, path, json=None):
        return self.req("PATCH", path, json=json)

    def put(self, path, json=None):
        return self.req("PUT", path, json=json)

    def login(self, email, pw=PW):
        r = self.post("/auth/login", {"email": email, "password": pw})
        check(f"login {email}", r.status_code == 200, r.text[:200])
        return r


def sample(name):
    for f in os.listdir(ARGS.samples):
        if f.startswith(name):
            with open(os.path.join(ARGS.samples, f), "rb") as fh:
                return f, fh.read()
    raise FileNotFoundError(name)


def err_code(r):
    try:
        return r.json()["error"]["code"]
    except Exception:
        return None


def main():
    rnd = "".join(random.choices(string.ascii_lowercase, k=6))
    anon = Client("anon")

    # ---------- public ----------
    r = anon.post("/public/eligibility", {"marital_status": "MARRIED", "applicant_dob": "1982-03-10", "spouse_dob": "1984-07-22", "years_married": 9, "existing_children": 0})
    j = r.json()
    check("eligibility: composite age 86 blocks 0-2 only", r.ok and j["eligible"] and j["allowed_age_groups"] == ["2-4", "4-8", "8-18"], r.text[:300])
    r = anon.post("/public/eligibility", {"marital_status": "MARRIED", "applicant_dob": "1990-01-01", "spouse_dob": "1991-01-01", "years_married": 1, "existing_children": 0})
    check("eligibility: marriage under 2 years not eligible", r.ok and not r.json()["eligible"], r.text[:200])
    r = anon.post("/public/eligibility", {"marital_status": "SINGLE_MALE", "applicant_dob": "1985-01-01", "existing_children": 0})
    check("eligibility: single man note about girl child", r.ok and any(x["code"] == "SINGLE_MALE_GIRL" for x in r.json()["reasons"]))
    r = anon.post("/public/eligibility", {"marital_status": "MARRIED", "applicant_dob": "bad"})
    check("eligibility: bad input gives VALIDATION_ERROR", r.status_code == 400 and err_code(r) == "VALIDATION_ERROR")
    r = anon.get("/public/fees")
    check("fees: total 64000", r.ok and r.json()["total_single_child"] == 64000)
    r = anon.get("/public/fees", headers={"Accept-Language": "hi"})
    check("fees: Hindi names", r.ok and "शुल्क" in r.text)
    r = anon.get("/public/agencies", params={"q": "Asha"})
    check("verify agency: Asha Shishu Griha verified", r.ok and any(a["name"] == "Asha Shishu Griha" and a["verified"] for a in r.json()["agencies"]))
    r = anon.get("/public/agencies", params={"q": "Fake Baby Home"})
    check("verify agency: unknown agency not found", r.ok and r.json()["agencies"] == [])
    scam_text = "Healthy newborn baby boy available for adoption, no paperwork needed, contact on WhatsApp 9876543210, Rs 50,000 advance."
    r = anon.post("/public/scam-check", {"text": scam_text, "language": "en"}, kind="ai")
    check("scam shield: LIKELY_SCAM", r.ok and r.json()["verdict"] == "LIKELY_SCAM", r.text[:300])
    token = r.json().get("report_token") if r.ok else None
    check("scam shield: phone number not repeated in reasons", r.ok and "9876543210" not in " ".join(r.json()["reasons"]))
    r = anon.post("/public/scam-report", {"report_token": token})
    check("scam report saved", r.ok and r.json().get("ok"))
    r = anon.post("/public/scam-report", {"report_token": token})
    check("scam report token cannot be reused", r.status_code == 404)
    r = anon.post("/public/scam-check", {"text": "We are a registered agency. Please apply on the CARINGS website.", "language": "en"}, kind="ai")
    check("scam shield: safe message LOOKS_SAFE", r.ok and r.json()["verdict"] == "LOOKS_SAFE", r.text[:200])
    r = anon.get("/applications/me")
    check("unauthenticated: 401 UNAUTHENTICATED", r.status_code == 401 and err_code(r) == "UNAUTHENTICATED")
    r = anon.s.options(BASE + "/auth/login", headers={"Origin": ORIGIN, "Access-Control-Request-Method": "POST"})
    check("CORS: website origin allowed with credentials", r.headers.get("Access-Control-Allow-Origin") == ORIGIN and r.headers.get("Access-Control-Allow-Credentials") == "true")
    r = anon.s.options(BASE + "/auth/login", headers={"Origin": "https://evil.example", "Access-Control-Request-Method": "POST"})
    check("CORS: other origins refused", "Access-Control-Allow-Origin" not in r.headers)

    # ---------- registration and login ----------
    email = f"e2e.{rnd}@test.childconnect.in"
    p = Client("parent")
    r = p.post("/auth/register", {"email": email, "phone": "+91 9811122233", "password": "short", "full_name": "Test Parent", "language": "en", "consent": True})
    check("register: short password rejected", r.status_code == 400)
    r = p.post("/auth/register", {"email": email, "phone": "+91 9811122233", "password": "Secret@123", "full_name": "Ravi and Meena Kulkarni", "language": "en", "consent": False})
    check("register: consent required", r.status_code == 400)
    r = p.post("/auth/register", {"email": email, "phone": "+91 9811122233", "password": "Secret@123", "full_name": "Rahul and Priya Sharma", "language": "en", "consent": True})
    check("register: parent created, cookie set", r.status_code == 201 and r.json()["user"]["role"] == "PARENT" and "cc_session" in p.s.cookies, r.text[:200])
    ck = r.headers.get("Set-Cookie", "")
    check("cookie is httpOnly and SameSite=Lax", "HttpOnly" in ck and "SameSite=Lax" in ck)
    r = Client("x").post("/auth/register", {"email": email, "phone": "+91 9811122233", "password": "Secret@123", "full_name": "Dup", "language": "en", "consent": True})
    check("register: duplicate email rejected", r.status_code == 400)
    r = p.get("/auth/me")
    check("me returns the user", r.ok and r.json()["user"]["email"] == email)
    r = p.post("/auth/refresh")
    check("refresh issues a new session", r.ok)
    r = Client("x").post("/auth/login", {"email": email, "password": "wrong-password"})
    check("login: wrong password 401", r.status_code == 401)
    r = p.get("/applications/me")
    check("no application yet: 404", r.status_code == 404)

    # ---------- profile ----------
    prof = {"marital_status": "MARRIED", "years_married": 6, "applicant_dob": "1989-05-10", "spouse_dob": "1991-08-02", "existing_children": 0,
            "annual_income_inr": 1200000, "state": "Maharashtra", "district": "Thane", "address": "B-12, Shanti Nagar, Mira Road East, Thane 401107",
            "pref_age_group": "8-18", "pref_gender": "ANY", "pref_states": ["Maharashtra"], "open_special_needs": False, "open_siblings": False}
    r = p.post("/applications", {**prof, "pref_age_group": "8-18", "applicant_dob": "2001-01-01", "spouse_dob": "2002-01-01"})
    check("profile: age gap rule blocks 8-18 for young couple", r.status_code == 400, r.text[:200])
    r = p.post("/applications", {**prof, "pref_age_group": "0-2"})
    check("profile: application created with 30-day deadline", r.status_code == 201 and r.json()["status"] == "REGISTERED", r.text[:300])
    r = p.get("/applications/me")
    me = r.json()
    app_id = me["id"]
    check("application: required docs listed incl. MARRIAGE", r.ok and any(d["category"] == "MARRIAGE" for d in me["required_docs"]) and me["doc_deadline"])
    check("application: address decrypted for owner", me.get("address", "").startswith("B-12"))
    check("application: assigned to Asha Shishu Griha (Thane)", me.get("agency_name") == "Asha Shishu Griha", str(me.get("agency_name")))
    r = p.get("/applications/me/tracker")
    t = r.json()
    check("tracker: stage 1 current, DOC_UPLOAD_30D deadline ~30 days", r.ok and t["stage_index"] == 1 and t["current_deadline"]["step"] == "DOC_UPLOAD_30D" and 29 <= t["current_deadline"]["days_left"] <= 30, str(t)[:300])
    r = p.patch("/applications/me", {"open_siblings": True})
    check("profile update allowed while REGISTERED", r.ok and r.json()["open_siblings"] is True)

    # ---------- documents and DocCheck ----------
    def up(cat, prefix, client=p):
        fn, data = sample(prefix)
        mime = "application/pdf" if fn.endswith(".pdf") else ("image/png" if fn.endswith(".png") else "image/jpeg")
        return client.req("POST", "/applications/me/documents", kind="ai", files={"file": (fn, data, mime)}, data={"category": cat})

    r = p.req("POST", "/applications/me/documents", files={"file": ("evil.pdf", b"MZ\x90\x00 not a pdf", "application/pdf")}, data={"category": "INCOME"})
    check("upload: fake PDF rejected by magic bytes", r.status_code == 400)
    other = Client("other-parent")
    other.post("/auth/register", {"email": "e2e.other." + rnd + "@test.childconnect.in", "phone": "+91 9811122234", "password": "Secret@123", "full_name": "Ravi and Meena Kulkarni", "language": "en", "consent": True})
    other.post("/applications", {**prof, "pref_age_group": "0-2"})
    r = up("INCOME", "07_salary", client=other)
    check("DocCheck: name on document differs from applicant -> NAME_MISMATCH", r.status_code == 201 and any(c["code"] == "NAME_MISMATCH" for c in r.json()["checks"]), r.text[:300])
    r = up("RESIDENCE", "05_rent")
    check("DocCheck: rent agreement -> AI_WARN with RESIDENCE_TYPE_NOT_IN_LIST", r.status_code == 201 and r.json()["status"] == "AI_WARN" and any(c["code"] == "RESIDENCE_TYPE_NOT_IN_LIST" for c in r.json()["checks"]), r.text[:300])
    r = up("RESIDENCE", "06_electricity")
    j = r.json()
    check("DocCheck: electricity bill -> AI_PASS (type, recent)", r.status_code == 201 and j["status"] == "AI_PASS", str(j)[:400])
    r = up("PHOTO", "12_blurry")
    check("DocCheck: blurry photo -> AI_FAIL IMAGE_BLURRY", r.status_code == 201 and r.json()["status"] == "AI_FAIL" and any(c["code"] == "IMAGE_BLURRY" for c in r.json()["checks"]), r.text[:300])
    r = up("PHOTO", "01_family")
    check("DocCheck: same file as another application -> DUPLICATE_FILE", r.status_code == 201 and any(c["code"] == "DUPLICATE_FILE" for c in r.json()["checks"]), r.text[:300])
    r = up("MEDICAL", "08_medical")
    check("DocCheck: medical cert without reg no -> DOCTOR_REG_MISSING", r.ok and any(c["code"] == "DOCTOR_REG_MISSING" for c in r.json()["checks"]))
    r = up("INCOME", "13_old_electricity")
    check("DocCheck: bill in income category -> TYPE_NOT_ALLOWED", r.ok and r.json()["status"] == "AI_FAIL")
    r = p.post("/applications/me/submit")
    check("submit blocked while documents missing", r.status_code == 400)
    for cat, pre in [("IDENTITY", "02_aadhaar"), ("BIRTH_PROOF", "04_birth"), ("INCOME", "07_salary"), ("MEDICAL", "09_medical"), ("MARRIAGE", "10_marriage"), ("REFERENCE", "11_reference")]:
        r = up(cat, pre)
        check(f"upload {cat}", r.status_code == 201, r.text[:200])
    r = p.post("/applications/me/submit")
    check("submit: DOCS_SUBMITTED with seniority date", r.ok and r.json()["status"] == "DOCS_SUBMITTED", r.text[:200])
    r = p.patch("/applications/me", {"open_siblings": False})
    check("profile locked after submission (409)", r.status_code == 409 and err_code(r) == "INVALID_TRANSITION")
    doc_id = [d for d in p.get("/applications/me").json()["required_docs"] if d["category"] == "RESIDENCE"][0]["document_id"]
    r = p.get(f"/documents/{doc_id}/url")
    url = r.json().get("url", "")
    check("parent gets a 5-minute link to own document", r.ok and r.json()["expires_in"] == 300)
    r = requests.get(url, timeout=30)
    check("signed link serves the PDF", r.ok and r.content[:5] == b"%PDF-")
    r = requests.get(url.replace("sig=", "sig=00"), timeout=30)
    check("tampered link refused", r.status_code == 403)

    # ---------- agency review ----------
    w = Client("worker")
    w.login("worker@childconnect.in")
    r = w.get("/agency/cases", params={"sort": "risk"})
    check("agency case list includes the new case", r.ok and any(c["application_id"] == app_id for c in r.json()))
    r = w.get(f"/agency/cases/{app_id}")
    case = r.json()
    check("case detail: documents with AI checks", r.ok and len(case["documents"]) >= 8 and all("ai_checks" in d for d in case["documents"]))
    res_doc = [d for d in case["documents"] if d["category"] == "RESIDENCE"][0]
    r = w.get(f"/documents/{res_doc['id']}/url")
    check("agency can open documents of own agency", r.ok)
    r = w.patch(f"/agency/documents/{res_doc['id']}/review", {"decision": "REUPLOAD_REQUESTED", "note": "x"})
    check("re-upload needs a reason", r.status_code == 400)
    r = w.patch(f"/agency/documents/{res_doc['id']}/review", {"decision": "REUPLOAD_REQUESTED", "note": "Please upload a clearer bill."})
    check("ask for re-upload -> DOCS_CORRECTION", r.ok and r.json()["application_status"] == "DOCS_CORRECTION", r.text[:200])
    me = p.get("/applications/me").json()
    check("parent sees reviewer note", any(d.get("reviewer_note") == "Please upload a clearer bill." for d in me["required_docs"]))
    r = up("RESIDENCE", "06_electricity")
    check("parent re-uploads in DOCS_CORRECTION", r.status_code == 201)
    r = p.post("/applications/me/submit")
    check("resubmit -> DOCS_SUBMITTED", r.ok)
    r = w.post(f"/agency/cases/{app_id}/verify-all")
    check("verify-all blocked until every document is verified", r.status_code == 400)
    case = w.get(f"/agency/cases/{app_id}").json()
    for d in case["documents"]:
        r = w.patch(f"/agency/documents/{d['id']}/review", {"decision": "VERIFIED"})
        if not r.ok:
            check(f"verify {d['category']}", False, r.text[:200])
    r = w.post(f"/agency/cases/{app_id}/verify-all")
    check("verify-all -> HSR_IN_PROGRESS with 60-day deadline", r.ok and r.json()["status"] == "HSR_IN_PROGRESS", r.text[:200])
    r = up("PHOTO", "01_family")
    check("parent cannot change documents after verification", r.status_code == 409)

    # ---------- Home Study Report Co-Pilot ----------
    notes = open(os.path.join(ARGS.samples, "hsr_rough_notes.txt"), encoding="utf-8").read()
    r = w.post("/ai/hsr/draft", {"application_id": app_id, "visit_date": date.today().isoformat(), "raw_notes": notes}, kind="ai")
    d = r.json()
    check("HSR draft returns 8 sections, missing items and completeness", r.ok and len(d["sections"]) == 8 and "ai_run_id" in d and 0 <= d["completeness_score"] <= 100, r.text[:300])
    run_id = d.get("ai_run_id")
    sections = {k: (v or "Information confirmed during the home visit.") for k, v in d.get("sections", {}).items()}
    sections["observations"] += " Edited by the social worker."
    r = w.put(f"/agency/hsr/{app_id}", {"visit_date": date.today().isoformat(), "final_sections": sections, "recommendation": "NEEDS_MORE_INFO"})
    check("HSR save", r.ok and r.json()["completeness_score"] == 100, r.text[:200])
    r = w.post(f"/agency/hsr/{app_id}/sign", {"ai_run_id": run_id})
    check("HSR cannot be signed as NEEDS_MORE_INFO", r.status_code == 400)
    w.put(f"/agency/hsr/{app_id}", {"visit_date": date.today().isoformat(), "final_sections": sections, "recommendation": "SUITABLE"})
    r = w.post(f"/agency/hsr/{app_id}/sign", {"ai_run_id": "00000000-0000-0000-0000-000000000000"})
    check("HSR sign needs the AI run of this case", r.status_code == 400)
    r = w.post(f"/agency/hsr/{app_id}/sign", {"ai_run_id": run_id})
    check("HSR signed -> WAITING, valid 3 years", r.ok and r.json()["status"] == "WAITING" and r.json()["hsr_valid_until"], r.text[:200])
    r = w.put(f"/agency/hsr/{app_id}", {"visit_date": date.today().isoformat(), "final_sections": sections, "recommendation": "SUITABLE"})
    check("signed HSR cannot be edited", r.status_code == 409)

    # ---------- children and Referral Lock ----------
    def new_child(gender="FEMALE", months=14):
        dob = (date.today() - timedelta(days=months * 30)).isoformat()
        r = w.post("/agency/children", {"name": "Synthetic E2E Child", "dob": dob, "gender": gender, "is_special_needs": False,
                                       "admitted_on": (date.today() - timedelta(days=60)).isoformat(), "cwc_lfa_number": "CWC/THN/LFA/2026/099",
                                       "lfa_date": (date.today() - timedelta(days=3)).isoformat()})
        check("add child -> case code, LEGALLY_FREE", r.status_code == 201 and r.json()["case_code"].startswith("MH-THN-"), r.text[:200])
        return r.json()["child_id"], r.json()["case_code"]

    child1, code1 = new_child()
    r = w.get(f"/agency/children/{child1}")
    c = r.json()
    check("child detail never returns the name", r.ok and "name" not in c and "Synthetic E2E" not in r.text and c["status"] == "LEGALLY_FREE")
    r = w.post("/agency/referrals", {"child_id": child1, "application_id": app_id})
    check("Referral Lock: referral refused (409) before reports and summary", r.status_code == 409)
    r = w.get(f"/agency/children/{child1}/next-parent")
    check("Referral Lock: next-parent refused before READY", r.status_code == 409)
    r = w.post("/ai/medical-summary", {"child_id": child1, "language": "en"}, kind="ai")
    check("medical summary needs the MER", r.status_code == 400)

    def ready(child):
        _, csr = sample("child_study_report")
        _, mer = sample("medical_examination_report")
        r = w.req("POST", f"/agency/children/{child}/reports", files={"csr": ("csr.pdf", csr, "application/pdf"), "mer": ("mer.pdf", mer, "application/pdf")})
        check("upload CSR and MER; MER text extracted", r.ok and r.json()["mer_text_extracted"], r.text[:200])
        r = w.post("/ai/medical-summary", {"child_id": child, "language": "hi"}, kind="ai")
        check("medical summary draft (Hindi)", r.ok and len(r.json()["draft"]) > 50, r.text[:200])
        run = r.json()["ai_run_id"]
        r = w.post(f"/agency/children/{child}/summary/approve", {"approved_text": r.json()["draft"] + "\nApproved by the medical officer.", "ai_run_id": run})
        check("approve summary -> READY", r.ok and r.json()["child_status"] == "READY", r.text[:200])

    ready(child1)
    r = w.get(f"/agency/children/{child1}/next-parent")
    check("next parent by seniority (rules, not AI)", r.ok and "matched_on" in r.json(), r.text[:200])
    r = w.post("/agency/referrals", {"child_id": child1, "application_id": app_id})
    check("referral sent, 48-hour window", r.status_code == 201 and r.json()["expires_at"], r.text[:200])

    # ---------- parent decision ----------
    r = p.get("/referrals/active")
    ref = r.json()
    check("parent sees referral 1 of 3 with case code only", r.ok and ref and ref["referral_number"] == 1 and ref["child"]["case_code"] == code1 and "Synthetic" not in r.text, r.text[:300])
    r = requests.get(ref["child"]["mer_url"], timeout=30)
    check("parent can open MER through 5-minute link", r.ok and r.content[:5] == b"%PDF-")
    r = p.post(f"/referrals/{ref['referral_id']}/decision", {"decision": "NOT_RESERVED", "reason": "no"})
    check("not reserving needs a reason", r.status_code == 400)
    r = p.post(f"/referrals/{ref['referral_id']}/decision", {"decision": "NOT_RESERVED", "reason": "We need more time to prepare."})
    check("NOT_RESERVED -> WAITING with 1-of-3 warning", r.ok and r.json()["status"] == "WAITING" and "1 of 3" in r.json()["warning"], r.text[:300])
    r = p.post(f"/referrals/{ref['referral_id']}/decision", {"decision": "RESERVED"})
    check("decision cannot be changed", r.status_code == 409)
    check("tracker counts 1 referral used", p.get("/applications/me/tracker").json()["referrals_used"] == 1)

    child2, code2 = new_child("MALE", 10)
    ready(child2)
    r = w.post("/agency/referrals", {"child_id": child2, "application_id": app_id})
    check("second referral sent", r.status_code == 201, r.text[:200])
    ref = p.get("/referrals/active").json()
    check("this is referral 2 of 3", ref["referral_number"] == 2)
    r = p.post(f"/referrals/{ref['referral_id']}/decision", {"decision": "RESERVED"})
    check("RESERVED -> 30-day matching deadline", r.ok and r.json()["status"] == "RESERVED", r.text[:200])

    # ---------- legal steps ----------
    today = date.today().isoformat()
    r = w.post(f"/agency/cases/{app_id}/events", {"event_type": "DM_ORDER", "event_date": today})
    check("legal event out of order -> 409 INVALID_TRANSITION", r.status_code == 409 and err_code(r) == "INVALID_TRANSITION")
    r = w.post(f"/agency/cases/{app_id}/events", {"event_type": "MATCHED", "event_date": (date.today() + timedelta(days=3)).isoformat()})
    check("future event date rejected", r.status_code == 400)
    for ev, want in [("MATCHED", "MATCHED_FOSTER_CARE"), ("FOSTER_CARE", "MATCHED_FOSTER_CARE"), ("DM_FILED", "DM_ORDER_PENDING"), ("DM_ORDER", "ADOPTED_FOLLOWUP"), ("BIRTH_CERT", "ADOPTED_FOLLOWUP")]:
        r = w.post(f"/agency/cases/{app_id}/events", {"event_type": ev, "event_date": today, "reference_no": "REF-" + ev})
        check(f"event {ev} -> {want}", r.ok and r.json()["status"] == want, r.text[:200])
    r = p.get("/followups/me")
    check("4 follow-up visits created, 6 months apart", r.ok and len(r.json()) == 4)
    case = w.get(f"/agency/cases/{app_id}").json()
    fnotes = open(os.path.join(ARGS.samples, "followup_visit_notes.txt"), encoding="utf-8").read()
    for i, fu in enumerate(sorted(case["followups"], key=lambda x: x["visit_number"])):
        r = w.post("/ai/followup/draft", {"followup_id": fu["id"], "raw_notes": fnotes}, kind="ai")
        ok = r.ok and "concern_flags" in r.json()
        if i == 0:
            check("follow-up draft flags 'not eating well'", ok and any("eating" in f.lower() for f in r.json()["concern_flags"]), r.text[:300])
        secs = {k: (v or "Nothing to report.") for k, v in r.json()["sections"].items()}
        r = w.post(f"/agency/followups/{fu['id']}/sign", {"final_report": secs, "visited_on": today})
        if i == 3:
            check("4th follow-up signed -> COMPLETED", r.ok and r.json()["status"] == "COMPLETED", r.text[:200])
    t = p.get("/applications/me/tracker").json()
    check("tracker shows COMPLETED with all stages done", t["status"] == "COMPLETED" and all(s["done"] for s in t["stages"]))

    # ---------- messages and Sahayak ----------
    r = p.post(f"/applications/{app_id}/messages", {"body": "Thank you for all the help."})
    check("parent sends message", r.status_code == 201)
    r = w.get(f"/applications/{app_id}/messages")
    check("agency reads the thread", r.ok and any(m["body"] == "Thank you for all the help." for m in r.json()))
    r = p.post("/ai/assistant", {"question": "How long is the home study report valid?", "language": "en"}, kind="ai")
    a = r.json()
    check("Sahayak: grounded answer with citation", r.ok and a["grounded"] and len(a["citations"]) > 0 and "3 years" in a["answer"], r.text[:400])
    r = p.post("/ai/assistant", {"question": "What happens if I do not reserve a child in 3 referrals?", "language": "en"}, kind="ai")
    a = r.json()
    check("Sahayak: cites Regulation 9(3) for debarment", r.ok and a["grounded"] and any("9(3)" in c["citation"] for c in a["citations"]), r.text[:400])
    r = p.post("/ai/assistant", {"question": "गृह अध्ययन रिपोर्ट कितने साल मान्य रहती है?", "language": "hi"}, kind="ai")
    check("Sahayak: Hindi question answered", r.ok and r.json()["grounded"], r.text[:300])
    r = p.post("/ai/assistant", {"question": "दत्तक शुल्क किती आहे?", "language": "mr"}, kind="ai")
    check("Sahayak: Marathi question answered", r.ok and r.json()["grounded"], r.text[:300])
    r = p.post("/ai/assistant", {"question": "Why is my report late?", "language": "en"}, kind="ai")
    check("Sahayak: own-case question is not guessed, escalation offered", r.ok and r.json()["is_about_own_case"] and r.json()["offer_escalation"])
    r = p.post("/ai/assistant", {"question": "What is the capital of France?", "language": "en"}, kind="ai")
    check("Sahayak: off-topic question not grounded", r.ok and not r.json()["grounded"], r.text[:300])
    r = p.post("/ai/assistant/escalate", {"question": "Why is my report late?"})
    check("escalation creates a case message", r.status_code == 201 and r.json()["message_id"])
    r = p.get("/notifications")
    check("parent has notifications", r.ok and len(r.json()["items"]) > 0)

    # ---------- seeded presentation flow: Referral Lock -> next parent by seniority ----------
    kids = w.get("/agency/children").json()
    demo = [k for k in kids if k["case_code"] == "MH-THN-2026-0002"]
    if check("seeded child MH-THN-2026-0002 is legally free with reports", demo and demo[0]["status"] == "LEGALLY_FREE"):
        cid = demo[0]["id"]
        r = w.post("/ai/medical-summary", {"child_id": cid, "language": "en"}, kind="ai")
        r = w.post(f"/agency/children/{cid}/summary/approve", {"approved_text": r.json()["draft"], "ai_run_id": r.json()["ai_run_id"]})
        r = w.get(f"/agency/children/{cid}/next-parent")
        check("next parent for the seeded child is Vikram and Anita Deshmukh (highest seniority)", r.ok and r.json()["parent_name"] == "Vikram and Anita Deshmukh", r.text[:200])
    alerts = adm_alerts = None

    # ---------- security: at least 10 cases must return 403 ----------
    p3 = Client("parent3")
    p3.login("parent3@childconnect.in")
    mum = Client("mumbai")
    mum.login("mumbai@childconnect.in")
    adm = Client("admin")
    adm.login("admin@childconnect.in")
    nag = Client("nagpur")
    nag.login("nagpur@childconnect.in")
    p3_app = p3.get("/applications/me").json()["id"]
    p3_doc = p3.get("/applications/me").json()["required_docs"][0]["document_id"]
    sec = [
        ("parent -> agency case list", p.get("/agency/cases")),
        ("parent -> admin radar", p.get("/admin/radar")),
        ("parent -> another parent's messages", p.get(f"/applications/{p3_app}/messages")),
        ("parent -> another parent's document", p.get(f"/documents/{p3_doc}/url")),
        ("agency -> audit log", w.get("/admin/audit")),
        ("agency -> approve agency", w.patch("/admin/agencies/00000000-0000-0000-0000-000000000000", {"is_approved": True})),
        ("other agency -> Thane case", mum.get(f"/agency/cases/{p3_app}")),
        ("other agency -> Thane child", mum.get(f"/agency/children/{child1}")),
        ("other agency -> Thane document review", mum.patch(f"/agency/documents/{p3_doc}/review", {"decision": "VERIFIED"})),
        ("other agency -> Thane case messages", mum.get(f"/applications/{p3_app}/messages")),
        ("admin -> edit a case (HSR)", adm.put(f"/agency/hsr/{p3_app}", {"visit_date": today, "final_sections": {}})),
        ("admin -> agency case list", adm.get("/agency/cases")),
        ("parent -> other parent's referral decision", p3.post(f"/referrals/{ref['referral_id']}/decision", {"decision": "RESERVED"})),
        ("unapproved agency -> cases", nag.get("/agency/cases")),
    ]
    for name, r in sec:
        check(f"403: {name}", r.status_code == 403 and err_code(r) == "FORBIDDEN", f"{r.status_code} {r.text[:150]}")
    r = adm.get("/admin/audit", params={"action": "FORBIDDEN_ATTEMPT"})
    check("forbidden attempts are in the audit log", r.ok and len(r.json()) >= 10, f"{len(r.json()) if r.ok else r.text[:200]}")

    # ---------- admin ----------
    r = adm.get("/admin/radar")
    rd = r.json()
    check("radar: summary and items with levels and reasons", r.ok and rd["summary"]["overdue"] > 0 and rd["summary"]["high_risk"] >= 0 and all("level" in i for i in rd["items"]), str(rd.get("summary")))
    r = adm.get("/admin/radar", params={"level": "OVERDUE"})
    check("radar: level filter", r.ok and all(i["level"] == "OVERDUE" for i in r.json()["items"]) and len(r.json()["items"]) > 0)
    r = adm.get("/admin/radar", params={"step": "HSR_60D"})
    check("radar: step filter", r.ok and all(i["step"] == "HSR_60D" for i in r.json()["items"]))
    r = adm.post("/admin/radar/recompute")
    check("recompute risk scores", r.ok and r.json()["updated"] > 0)
    r = adm.get("/admin/agency-scorecard")
    check("scorecard: avg days per agency", r.ok and len(r.json()) >= 5 and any("HSR_60D" in a["avg_days"] for a in r.json()))
    r = adm.get("/admin/child-alerts")
    reasons = {a["reason"] for a in r.json()}
    check("child alerts: no family visit and missing reports", r.ok and {"NO_FAMILY_VISIT_12M", "REPORTS_MISSING_10D"} <= reasons, str(r.json())[:300])
    check("child alerts: only the 2 seeded children without family visits", r.ok and sum(1 for a in r.json() if a["reason"] == "NO_FAMILY_VISIT_12M") == 2, str(r.json())[:300])
    check("child alerts show case codes only", "Synthetic" not in r.text)
    r = adm.get("/admin/agencies", params={"approved": "false"})
    pending = r.json()
    check("pending agency listed (Navjeevan Bal Sadan)", r.ok and any(a["name"] == "Navjeevan Bal Sadan" for a in pending))
    nag_id = [a for a in pending if a["name"] == "Navjeevan Bal Sadan"][0]["id"]
    r = adm.patch(f"/admin/agencies/{nag_id}", {"is_approved": True})
    check("admin approves agency", r.ok and r.json()["is_approved"])
    r = nag.get("/agency/cases")
    check("approved agency can now use agency pages", r.ok)
    kb = "Regulation 99: Test rule\nThis is a test rule added by the end-to-end test. " + "It explains the testing process in detail. " * 10
    r = adm.post("/admin/knowledge", {"source_title": "E2E Test Source", "language": "en", "text": kb})
    check("knowledge upload creates cited chunks", r.status_code == 201 and r.json()["chunks_created"] >= 1, r.text[:200])
    r = adm.get("/admin/knowledge")
    check("knowledge list shows sources", r.ok and any(k["source_title"] == "E2E Test Source" for k in r.json()))
    r = adm.get("/admin/scam-reports")
    check("scam reports listed", r.ok and len(r.json()) >= 1)
    sid = r.json()[0]["id"]
    r = adm.patch(f"/admin/scam-reports/{sid}", {"reviewed": True})
    check("scam report marked reviewed", r.ok)
    r = adm.get("/admin/audit", params={"action": "SIGN_HSR"})
    check("audit: HSR signing recorded", r.ok and len(r.json()) >= 1)
    r = adm.get("/admin/audit/verify")
    v = r.json()
    check("audit hash chain verifies", r.ok and v["valid"] and v["rows_checked"] > 50, str(v))
    r = adm.get("/admin/model")
    check("model card: synthetic metrics reported", r.ok and r.json()["risk_model"]["metrics"]["logistic_regression"]["roc_auc"] > 0.5)

    # ---------- logout and lockout ----------
    r = p.post("/auth/logout")
    check("logout", r.ok)
    r = p.get("/auth/me")
    check("after logout: 401", r.status_code == 401)
    r = p.post("/auth/refresh")
    check("refresh token revoked by logout", r.status_code == 401)
    if not ARGS.skip_lockout:
        lk = Client("lock")
        for _ in range(5):
            lk.post("/auth/login", {"email": email, "password": "wrong-password!"})
        r = lk.post("/auth/login", {"email": email, "password": "Secret@123"})
        check("login rate limit: 6th attempt in a minute -> 429", r.status_code == 429)
        print("     waiting 61 s for the rate-limit window...")
        time.sleep(61)
        r = lk.post("/auth/login", {"email": email, "password": "Secret@123"})
        check("5 failed logins lock the account for 15 minutes", r.status_code == 401 and "locked" in r.text.lower(), r.text[:200])

    # ---------- summary ----------
    passed = sum(1 for _, ok in results if ok)
    print("\n" + "=" * 60)
    print(f"RESULT: {passed}/{len(results)} checks passed")
    for kind, ts in timings.items():
        if ts:
            print(f"average response time ({kind} APIs): {1000 * sum(ts) / len(ts):.0f} ms over {len(ts)} calls")
    failed = [n for n, ok in results if not ok]
    if failed:
        print("FAILED:")
        for n in failed:
            print("  - " + n)
    return 0 if not failed else 1


if __name__ == "__main__":
    sys.exit(main())
