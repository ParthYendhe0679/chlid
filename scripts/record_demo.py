"""Records an end-to-end demo video of Child Connect (all roles) with Playwright.

Prerequisites: API on :8080 and website on :3000, freshly seeded data (scripts/reset-data.ps1).
Usage: python scripts/record_demo.py  -> demo-video/child-connect-demo.mp4
"""
import os
import shutil
import subprocess
import sys
import time
from datetime import date, timedelta

from playwright.sync_api import sync_playwright

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SAMPLES = os.path.join(ROOT, "sample-documents")
OUT = os.path.join(ROOT, "demo-video")
WEB = "http://localhost:3000"
API = "http://localhost:8080/api/v1"
PW = "ChildConnect@2026"
W, H = 1440, 900

OVERLAY_JS = r"""
(() => {
  if (window.__ccOverlay) return; window.__ccOverlay = true;
  const style = document.createElement('style');
  style.textContent = `
    #cc-cursor{position:fixed;z-index:2147483647;width:22px;height:22px;margin:-4px 0 0 -4px;pointer-events:none;
      transition:left .45s cubic-bezier(.2,.7,.2,1),top .45s cubic-bezier(.2,.7,.2,1);}
    #cc-cursor svg{filter:drop-shadow(0 2px 3px rgba(0,0,0,.35))}
    #cc-ring{position:fixed;z-index:2147483646;width:36px;height:36px;margin:-18px 0 0 -18px;border-radius:50%;
      border:3px solid #7b5cf0;opacity:0;pointer-events:none;transform:scale(.4)}
    #cc-ring.go{animation:ccring .5s ease-out}
    @keyframes ccring{0%{opacity:.9;transform:scale(.4)}100%{opacity:0;transform:scale(1.6)}}
    #cc-cap{position:fixed;left:50%;bottom:26px;transform:translateX(-50%) translateY(20px);z-index:2147483645;
      max-width:1100px;padding:14px 26px;border-radius:18px;background:rgba(28,27,43,.92);color:#fff;
      font:600 19px/1.4 Poppins,system-ui,sans-serif;box-shadow:0 12px 30px rgba(0,0,0,.25);opacity:0;
      transition:opacity .4s ease, transform .4s ease;pointer-events:none;text-align:center}
    #cc-cap.on{opacity:1;transform:translateX(-50%) translateY(0)}
    #cc-cap b{color:#ffd166}
    #cc-title{position:fixed;inset:0;z-index:2147483644;display:flex;flex-direction:column;align-items:center;justify-content:center;
      background:linear-gradient(135deg,#2a2160,#5b3fd6 55%,#f08a6c);color:#fff;opacity:0;transition:opacity .6s ease;
      font-family:Poppins,system-ui,sans-serif;pointer-events:none;text-align:center;padding:40px}
    #cc-title.on{opacity:1}
    #cc-title h1{font:800 54px/1.15 Unbounded,Poppins,sans-serif;margin:0 0 18px}
    #cc-title p{font:500 24px/1.5 Poppins,sans-serif;margin:0;max-width:980px;opacity:.95}
    #cc-title .tag{margin-top:26px;padding:8px 18px;border-radius:999px;background:rgba(255,255,255,.18);font:600 16px Poppins}
  `;
  document.documentElement.appendChild(style);
  const c = document.createElement('div'); c.id = 'cc-cursor';
  c.innerHTML = '<svg width="22" height="22" viewBox="0 0 24 24"><path d="M3 2l7.5 19 2.6-7.7L21 10.8z" fill="#1c1b2b" stroke="#fff" stroke-width="1.6"/></svg>';
  const ring = document.createElement('div'); ring.id = 'cc-ring';
  const cap = document.createElement('div'); cap.id = 'cc-cap';
  const title = document.createElement('div'); title.id = 'cc-title';
  document.documentElement.append(c, ring, cap, title);
  const pos = JSON.parse(sessionStorage.getItem('ccpos') || '[720,450]');
  c.style.left = pos[0] + 'px'; c.style.top = pos[1] + 'px';
  window.__ccMove = (x, y) => { c.style.left = x + 'px'; c.style.top = y + 'px'; sessionStorage.setItem('ccpos', JSON.stringify([x, y])); };
  window.__ccClick = (x, y) => { ring.style.left = x + 'px'; ring.style.top = y + 'px'; ring.classList.remove('go'); void ring.offsetWidth; ring.classList.add('go'); };
  window.__ccCap = (html) => { if (!html) { cap.classList.remove('on'); return; } cap.innerHTML = html; cap.classList.add('on'); };
  window.__ccTitle = (h, p, tag) => { if (!h) { title.classList.remove('on'); return; }
    title.innerHTML = '<h1>' + h + '</h1><p>' + (p || '') + '</p>' + (tag ? '<div class="tag">' + tag + '</div>' : ''); title.classList.add('on'); };
  const saved = sessionStorage.getItem('cccap'); if (saved) { cap.innerHTML = saved; cap.classList.add('on'); }
})();
"""


class Demo:
    def __init__(self, page):
        self.p = page
        self.cap_text = ""

    def ensure(self):
        try:
            self.p.evaluate(OVERLAY_JS)
            if self.cap_text:
                self.p.evaluate("t => window.__ccCap(t)", self.cap_text)
        except Exception:
            pass

    def wait(self, s):
        time.sleep(s)

    def caption(self, html, hold=0.0):
        self.cap_text = html
        self.ensure()
        self.p.evaluate("t => { sessionStorage.setItem('cccap', t); window.__ccCap(t); }", html)
        self.wait(hold)

    def title(self, h, sub="", tag="", hold=3.2):
        self.ensure()
        self.p.evaluate("([a,b,c]) => window.__ccTitle(a,b,c)", [h, sub, tag])
        self.wait(hold)
        self.p.evaluate("() => window.__ccTitle('')")
        self.wait(0.7)

    def goto(self, path, settle=1.6):
        self.p.goto(WEB + path, wait_until="networkidle")
        self.ensure()
        self.wait(settle)

    def move_to(self, loc):
        loc.scroll_into_view_if_needed()
        self.smooth_center(loc)
        box = loc.bounding_box()
        x, y = box["x"] + min(box["width"] / 2, 120), box["y"] + box["height"] / 2
        self.p.evaluate("([x,y]) => window.__ccMove && window.__ccMove(x,y)", [x, y])
        self.p.mouse.move(x, y, steps=12)
        self.wait(0.55)
        return x, y

    def smooth_center(self, loc):
        try:
            loc.evaluate("el => { const r = el.getBoundingClientRect(); if (r.top < 90 || r.bottom > innerHeight - 120) "
                         "window.scrollBy({top: r.top - innerHeight/2 + r.height/2, behavior: 'smooth'}); }")
            self.wait(0.7)
        except Exception:
            pass

    def click(self, loc, after=0.8):
        self.ensure()
        x, y = self.move_to(loc)
        self.p.evaluate("([x,y]) => window.__ccClick && window.__ccClick(x,y)", [x, y])
        loc.click()
        self.wait(after)

    def type(self, loc, text, delay=28):
        self.click(loc, after=0.2)
        loc.fill("")
        loc.press_sequentially(text, delay=delay)
        self.wait(0.4)

    def fill(self, loc, text):
        self.click(loc, after=0.2)
        loc.fill(text)
        self.wait(0.5)

    def scroll(self, px, pause=1.4):
        self.p.evaluate("y => window.scrollBy({top: y, behavior: 'smooth'})", px)
        self.wait(pause)

    def top(self):
        self.p.evaluate("() => window.scrollTo({top: 0, behavior: 'smooth'})")
        self.wait(0.8)

    def btn(self, name, exact=False):
        return self.p.get_by_role("button", name=name, exact=exact).first

    def login(self, email, label):
        self.p.context.clear_cookies()
        self.goto("/login", settle=0.8)
        self.caption(f"Log in as <b>{label}</b>")
        self.type(self.p.locator("#email"), email, delay=18)
        self.type(self.p.locator("#password"), PW, delay=18)
        self.click(self.p.get_by_role("button", name="Log in", exact=True), after=0.3)
        self.p.wait_for_load_state("networkidle")
        self.wait(2.2)
        self.ensure()

    def upload(self, cat, filename, wait_text=None, hold=2.5):
        inp = self.p.locator(f"#file-{cat}")
        label = self.p.locator(f"label[for='file-{cat}']")
        self.move_to(label)
        self.p.evaluate("([x,y]) => 0", [0, 0])
        inp.set_input_files(os.path.join(SAMPLES, filename))
        try:
            self.p.get_by_text("Close results").first.wait_for(timeout=90000)
        except Exception:
            pass
        self.wait(0.6)
        res = self.p.get_by_text("Close results").first
        try:
            self.smooth_center(res)
        except Exception:
            pass
        self.wait(hold)

    def close_results(self):
        for b in self.p.get_by_role("button", name="Close results").all():
            try:
                b.click()
            except Exception:
                pass
        self.wait(0.4)


def run():
    os.makedirs(OUT, exist_ok=True)
    tmp = os.path.join(OUT, "raw")
    shutil.rmtree(tmp, ignore_errors=True)
    with sync_playwright() as pw:
        browser = pw.chromium.launch(args=["--force-device-scale-factor=1"])
        ctx = browser.new_context(viewport={"width": W, "height": H}, record_video_dir=tmp, record_video_size={"width": W, "height": H},
                                  locale="en-IN", timezone_id="Asia/Kolkata")
        ctx.add_init_script(OVERLAY_JS.replace("(() => {", "window.addEventListener('DOMContentLoaded', () => {").replace("})();", "});"))
        page = ctx.new_page()
        page.set_default_timeout(120000)
        d = Demo(page)
        try:
            script(d, page)
        finally:
            video = page.video.path() if page.video else None
            ctx.close()
            browser.close()
    return video


def script(d, p):
    # ---------- intro ----------
    d.goto("/", settle=0.5)
    d.title("Child Connect", "A secure helper for India's legal adoption process. AI helps with paperwork and deadlines. A human always decides.",
            "Puran Poli, TSEC  |  Next.js + Go + Neon PostgreSQL + Gemini", hold=4.5)
    d.caption("Public home page: anyone can use these tools <b>without logging in</b>", 2)
    d.scroll(650, 1.8)
    d.scroll(650, 1.8)
    d.top()

    # ---------- public tools ----------
    d.title("1. Public tools", "Eligibility Checker, official Fee Card, Verify Agency and Scam Shield", hold=2.6)
    d.goto("/eligibility")
    d.caption("<b>Eligibility Checker</b>: the 2022 rules on composite age, marriage duration and children")
    p.locator("#marital_status").select_option("MARRIED")
    d.fill(p.locator("#applicant_dob"), "1982-03-10")
    d.fill(p.locator("#spouse_dob"), "1984-07-22")
    d.fill(p.locator("#years_married"), "9")
    d.fill(p.locator("#existing_children"), "0")
    d.click(d.btn("Check eligibility"), after=2.2)
    d.caption("Combined age 86 is over the limit of 85 for children up to 2 years, so only <b>older age groups</b> are allowed, with the reason")
    d.scroll(380, 3.2)

    d.goto("/fees")
    d.caption("<b>Fee Card</b>: official, non-refundable fees from the server. No extra payment is ever required", 3.2)

    d.goto("/verify-agency")
    d.caption("<b>Verify Agency</b>: check that an agency is registered before trusting it")
    d.type(p.locator("#agency-q"), "Asha", delay=60)
    d.click(d.btn("Search"), after=2.6)

    d.goto("/scam-shield")
    d.caption("<b>Scam Shield</b>: rule layer in Go + Gemini AI layer check a suspicious adoption post")
    d.type(p.locator("#text"), "Healthy newborn baby boy available for adoption, no paperwork needed, contact on WhatsApp 98XXXXXX21, Rs 50,000 advance.", delay=12)
    d.click(d.btn("Check message"), after=0.5)
    p.get_by_text("Report this message").first.wait_for(timeout=90000)
    d.wait(1.0)
    d.caption("Verdict <b>LIKELY SCAM</b> with reasons and the law. The user can report it to the admin")
    d.scroll(420, 3.0)
    d.click(p.get_by_role("button", name="Report this message").first, after=2.0)

    # ---------- parent ----------
    d.title("2. Parent journey", "Rahul and Priya Sharma: registered, 12 days left to upload documents", "parent@childconnect.in", hold=2.8)
    d.login("parent@childconnect.in", "parent: Rahul and Priya Sharma")
    d.caption("<b>Case Tracker</b>: current stage, days left on the legal deadline, and what happens next", 2.8)
    d.scroll(520, 2.6)
    d.top()

    d.goto("/parent/documents")
    d.caption("<b>DocCheck AI</b>: every upload is checked at once. Rent agreement as residence proof...")
    d.upload("RESIDENCE", "05_rent_agreement_(expect_warning).pdf", hold=3.2)
    d.caption("<b>Warning</b>: a rent agreement is not in the usual list of residence proofs. The parent knows early, not after 30 days")
    d.wait(2.0)
    d.close_results()
    d.caption("Now the electricity bill: type, name match, address match and bill date are checked")
    d.upload("RESIDENCE", "06_electricity_bill.pdf", hold=3.5)
    d.close_results()
    d.caption("A blurry photo is rejected by the <b>image-quality check in Go</b> (variance of Laplacian)")
    d.upload("PHOTO", "12_blurry_photo_(expect_fail).jpg", hold=3.0)
    d.close_results()
    d.caption("Uploading the remaining documents...")
    for cat, f in [("PHOTO", "01_family_photo.jpg"), ("INCOME", "07_salary_slip.pdf"), ("MEDICAL", "09_medical_certificate.pdf"),
                   ("MARRIAGE", "10_marriage_certificate.pdf"), ("REFERENCE", "11_reference_letter.pdf")]:
        d.upload(cat, f, hold=1.2)
        d.close_results()
    d.top()
    d.caption("All documents uploaded. The parent submits, even with warnings: <b>a human makes the final decision</b>")
    d.click(d.btn("Submit documents"), after=1.2)
    d.click(d.btn("Submit now"), after=2.5)

    d.goto("/parent/assistant")
    d.caption("<b>Sahayak AI</b>: answers only from official rules, with citations, in English, Hindi and Marathi")
    box = p.get_by_placeholder("Type your question...")
    d.type(box, "What happens if I do not reserve the child?", delay=30)
    d.click(d.btn("Send", exact=True), after=0.5)
    p.get_by_text("Sources (").first.wait_for(timeout=90000)
    d.wait(3.0)
    d.click(p.get_by_role("button", name="मराठी").first, after=0.6)
    d.type(box, "दत्तक शुल्क किती आहे?", delay=60)
    d.click(d.btn("Send", exact=True), after=0.5)
    p.get_by_text("Sources (").nth(1).wait_for(timeout=90000)
    d.caption("Answer in <b>Marathi</b>, grounded in the official text, with the citation shown")
    d.wait(3.5)

    # ---------- agency ----------
    d.title("3. Agency journey", "Meera Kulkarni, social worker at Asha Shishu Griha, Thane", "worker@childconnect.in", hold=2.8)
    d.login("worker@childconnect.in", "social worker: Meera Kulkarni")
    d.caption("<b>My cases</b>: most urgent first, with the ML-predicted risk and its top reasons", 3.0)
    d.scroll(450, 2.2)
    d.top()
    d.click(p.get_by_text("Sanjay and Neha Joshi").first, after=2.5)
    d.ensure()
    d.caption("<b>Document review</b>: the document next to the AI findings. The social worker decides")
    d.click(d.btn("Show document"), after=2.4)
    d.click(p.get_by_role("button", name="Verify", exact=True).first, after=2.0)
    d.scroll(1300, 2.0)
    d.caption("The rent agreement gets a <b>re-upload request</b> with a reason the parent will see")
    reup = p.get_by_role("button", name="Ask for re-upload").nth(3)
    d.click(reup, after=1.0)
    d.type(p.locator("#note"), "Please upload a recent electricity or telephone bill as residence proof.", delay=15)
    d.click(d.btn("Send request"), after=2.4)

    cases = p.evaluate("""async () => (await (await fetch('%s/agency/cases', {credentials:'include'})).json())""" % API)
    shaikh = [c for c in cases if c["parent_name"].startswith("Imran")][0]["application_id"]
    d.goto(f"/agency/cases/{shaikh}")
    d.caption("<b>Report Co-Pilot</b>: rough home-visit notes become the official Home Study Report sections")
    d.click(p.get_by_role("tab", name="Home Study Report"), after=1.0)
    d.fill(p.locator("#visit_date"), (date.today() - timedelta(days=2)).isoformat())
    notes = open(os.path.join(SAMPLES, "hsr_rough_notes.txt"), encoding="utf-8").read()
    d.click(p.locator("#raw_notes"), after=0.2)
    p.locator("#raw_notes").fill(notes)
    d.wait(1.2)
    d.click(d.btn("Draft with AI"), after=0.6)
    d.caption("Gemini arranges the notes into sections, adds no facts, and lists <b>missing items</b>")
    p.locator("#sec-family_background").wait_for(timeout=10000)
    p.wait_for_function("() => document.querySelector('#sec-family_background') && document.querySelector('#sec-family_background').value.length > 5", timeout=120000)
    d.wait(1.5)
    d.scroll(500, 2.5)
    d.scroll(500, 2.5)
    d.caption("The social worker edits, chooses the recommendation, saves and <b>signs</b>")
    sup = p.locator("#sec-support_system")
    if not sup.input_value().strip():
        d.type(sup, "Imran's parents live in the same building and will help with childcare.", delay=12)
    d.click(p.locator("#recommendation"), after=0.2)
    p.locator("#recommendation").select_option("SUITABLE")
    d.wait(0.6)
    d.click(d.btn("Save draft"), after=2.0)
    d.click(d.btn("Sign and submit"), after=1.0)
    d.click(p.get_by_role("button", name="Sign", exact=True), after=3.0)
    d.top()
    d.caption("Signed: the case moves to the <b>waiting list</b>; the report is valid for 3 years", 2.5)

    kids = p.evaluate("""async () => (await (await fetch('%s/agency/children', {credentials:'include'})).json())""" % API)
    kid = [k for k in kids if k["case_code"] == "MH-THN-2026-0002"][0]["id"]
    d.goto("/agency/children")
    d.caption("<b>Children</b> are shown by case code only. Names are encrypted and never displayed", 2.6)
    d.goto(f"/agency/children/{kid}")
    d.caption("<b>Referral Lock</b>: a child can be referred only after the CSR, the MER and an approved summary exist")
    d.wait(1.5)
    d.click(d.btn("Draft with AI"), after=0.5)
    d.caption("Gemini explains the medical report to parents in simple, kind words, without diagnosing")
    p.locator("#summary").wait_for(timeout=120000)
    d.wait(1.0)
    d.smooth_center(p.locator("#summary"))
    d.wait(3.0)
    d.click(d.btn("Approve summary"), after=2.5)
    d.caption("Summary approved: the child is <b>Ready for referral</b>. The next parent comes from the seniority queue (rules, not AI)")
    d.click(d.btn("Find next parent"), after=3.0)
    d.click(d.btn("Send referral", exact=True), after=2.5)

    # ---------- parent 2 ----------
    d.title("4. Referral and the 48-hour decision", "Vikram and Anita Deshmukh, highest seniority in the waiting list", "parent2@childconnect.in", hold=2.8)
    d.login("parent2@childconnect.in", "parent: Vikram and Anita Deshmukh")
    d.goto("/parent/referral")
    d.caption("Case code, age, approved medical summary and reports, with the <b>48-hour timer</b> and the 1-of-3 warning", 3.0)
    d.scroll(450, 2.6)
    d.top()
    d.click(d.btn("Reserve this child"), after=1.2)
    d.click(d.btn("Yes, reserve"), after=3.0)
    d.goto("/parent")
    d.caption("The tracker now shows the reservation and the <b>30-day matching deadline</b>", 3.0)

    # ---------- admin ----------
    d.title("5. Government oversight", "Kavita Rao, state oversight officer", "admin@childconnect.in", hold=2.6)
    d.login("admin@childconnect.in", "admin: Kavita Rao")
    d.caption("<b>Deadline Radar</b>: overdue is a fact; high risk is a prediction from a logistic regression model with reasons", 3.2)
    d.scroll(560, 3.0)
    d.scroll(900, 3.0)
    d.caption("<b>Agency scorecard</b>: average days per step against the 60-day legal limit", 2.6)
    d.top()
    d.goto("/admin/child-alerts")
    d.caption("<b>Child Review Alerts</b>: no family visit for 12 months, or reports missing after 10 days", 3.0)
    d.goto("/admin/agencies")
    d.caption("<b>Agency approval</b>: a new agency cannot use the system until an officer approves it")
    d.click(d.btn("Approve"), after=2.4)
    d.goto("/admin/scams")
    d.caption("Messages reported through <b>Scam Shield</b> reach the admin", 3.0)
    d.goto("/admin/audit")
    d.caption("<b>Audit log</b>: every sensitive action in a SHA-256 hash chain. Verify proves nothing was changed")
    d.click(d.btn("Verify chain"), after=3.5)

    d.title("Faster, clearer and safer", "AI never ranks children. A human always decides. Built to work alongside CARINGS.",
            "Child Connect  |  Puran Poli, TSEC", hold=4.5)


def to_mp4(webm, mp4):
    try:
        import imageio_ffmpeg
        ff = imageio_ffmpeg.get_ffmpeg_exe()
    except Exception:
        return None
    subprocess.run([ff, "-y", "-i", webm, "-c:v", "libx264", "-preset", "medium", "-crf", "20", "-pix_fmt", "yuv420p",
                    "-r", "30", "-movflags", "+faststart", mp4], check=True, capture_output=True)
    return mp4


if __name__ == "__main__":
    start = time.time()
    webm = run()
    final_webm = os.path.join(OUT, "child-connect-demo.webm")
    shutil.copy(webm, final_webm)
    mp4 = to_mp4(final_webm, os.path.join(OUT, "child-connect-demo.mp4"))
    print("video:", mp4 or final_webm, f"({time.time() - start:.0f}s)")
    sys.exit(0)
