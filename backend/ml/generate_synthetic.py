"""Generate SYNTHETIC deadline data for the Deadline Radar risk model (guide section 8.4).

The data is artificial. Delay patterns follow what the 2023 CAG Delhi audit and news reports describe:
home study reports and District Magistrate orders run late most often, overloaded agencies with few
staff are slower, and cases with document re-uploads or slow parent replies slip more.

Usage: python generate_synthetic.py [n_cases] > synthetic_deadlines.csv
"""
import csv
import sys

import numpy as np

STEPS = ["DOC_UPLOAD_30D", "HSR_60D", "CSR_MER_10D", "RESERVE_48H", "MATCHING_30D",
         "FOSTER_CARE_10D", "DM_FILING_10D", "DM_ORDER_60D", "FOLLOWUP_VISIT"]
# Relative tendency of each step to run late (log-odds), based on the audit findings.
STEP_EFFECT = {"DOC_UPLOAD_30D": -0.4, "HSR_60D": 0.9, "CSR_MER_10D": 0.7, "RESERVE_48H": -1.2,
               "MATCHING_30D": 0.1, "FOSTER_CARE_10D": -0.3, "DM_FILING_10D": 0.3, "DM_ORDER_60D": 1.0,
               "FOLLOWUP_VISIT": 0.5}
STEP_P = [0.10, 0.22, 0.08, 0.04, 0.08, 0.06, 0.08, 0.16, 0.18]

FEATURES = ["days_elapsed_ratio", "agency_open_cases", "agency_staff_count", "agency_past_late_rate",
            "docs_reupload_count", "step_HSR", "step_CSR_MER", "step_MATCHING", "step_DM_FILING",
            "step_DM_ORDER", "step_FOLLOWUP", "parent_reply_delay_days"]


def generate(n, seed=42):
    rng = np.random.default_rng(seed)
    rows = []
    # 120 synthetic agencies with their own staffing and habits.
    n_ag = 120
    staff = rng.integers(1, 9, n_ag)
    open_cases = np.clip((staff * rng.gamma(4.0, 2.2, n_ag)).astype(int), 3, 90)
    habit = rng.normal(0, 0.6, n_ag)  # hidden agency efficiency
    past_late = np.clip(1 / (1 + np.exp(-(-1.0 + habit + 0.03 * (open_cases / staff)))) + rng.normal(0, 0.05, n_ag), 0, 1)
    for _ in range(n):
        a = rng.integers(0, n_ag)
        step = rng.choice(STEPS, p=STEP_P)
        ratio = float(np.clip(rng.beta(2.0, 2.2) * 1.3, 0, 1.4))
        reup = int(rng.poisson(0.6 if step in ("HSR_60D", "DOC_UPLOAD_30D") else 0.25))
        reply = float(np.clip(rng.gamma(1.6, 1.6), 0, 20))
        load = open_cases[a] / staff[a]
        logit = (-3.1 + 3.4 * ratio + 0.045 * load - 0.08 * staff[a] + 2.2 * past_late[a] + 0.45 * reup
                 + 0.11 * reply + STEP_EFFECT[step] + 0.5 * habit[a] + rng.normal(0, 0.5))
        missed = int(rng.random() < 1 / (1 + np.exp(-logit)))
        row = {
            "days_elapsed_ratio": round(ratio, 4),
            "agency_open_cases": int(open_cases[a]),
            "agency_staff_count": int(staff[a]),
            "agency_past_late_rate": round(float(past_late[a]), 4),
            "docs_reupload_count": reup,
            "step_HSR": int(step == "HSR_60D"),
            "step_CSR_MER": int(step == "CSR_MER_10D"),
            "step_MATCHING": int(step == "MATCHING_30D"),
            "step_DM_FILING": int(step == "DM_FILING_10D"),
            "step_DM_ORDER": int(step == "DM_ORDER_60D"),
            "step_FOLLOWUP": int(step == "FOLLOWUP_VISIT"),
            "parent_reply_delay_days": round(reply, 2),
            "step": step,
            "missed": missed,
        }
        rows.append(row)
    return rows


if __name__ == "__main__":
    n = int(sys.argv[1]) if len(sys.argv) > 1 else 5000
    rows = generate(n)
    w = csv.DictWriter(sys.stdout, fieldnames=FEATURES + ["step", "missed"], lineterminator="\n")
    w.writeheader()
    w.writerows(rows)
