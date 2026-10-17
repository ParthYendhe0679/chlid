"""Train the Deadline Radar risk model (guide section 8.4) on SYNTHETIC data and export model.json.

Logistic Regression is the main model (simple, explainable). A small Gradient Boosting model is trained
only for comparison in the paper. The Go server reads model.json and computes
sigmoid(intercept + sum(coef * (x - mean) / scale)) itself; no Python runs in production.

Usage: python train_risk.py  (writes ../internal/ai/risk/model.json and prints the metrics)
"""
import json
import os
import sys
from datetime import date

import numpy as np
from sklearn.ensemble import GradientBoostingClassifier
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import accuracy_score, precision_score, recall_score, roc_auc_score
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import StandardScaler

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from generate_synthetic import FEATURES, generate  # noqa: E402

THRESHOLD = 0.6  # "High risk" level in the guide


def metrics(y, p):
    pred = (p >= 0.5).astype(int)
    return {
        "accuracy": round(float(accuracy_score(y, pred)), 4),
        "precision": round(float(precision_score(y, pred)), 4),
        "recall": round(float(recall_score(y, pred)), 4),
        "roc_auc": round(float(roc_auc_score(y, p)), 4),
    }


def main():
    rows = generate(5000)
    X = np.array([[r[f] for f in FEATURES] for r in rows], dtype=float)
    y = np.array([r["missed"] for r in rows])
    X_tr, X_te, y_tr, y_te = train_test_split(X, y, test_size=0.25, random_state=7, stratify=y)

    scaler = StandardScaler().fit(X_tr)
    lr = LogisticRegression(max_iter=1000).fit(scaler.transform(X_tr), y_tr)
    p_lr = lr.predict_proba(scaler.transform(X_te))[:, 1]

    gb = GradientBoostingClassifier(n_estimators=150, max_depth=3, random_state=7).fit(X_tr, y_tr)
    p_gb = gb.predict_proba(X_te)[:, 1]

    model = {
        "name": "deadline-risk-logreg",
        "version": "1.0",
        "trained_on": date.today().isoformat(),
        "data": "SYNTHETIC: 5000 generated deadlines (ml/generate_synthetic.py). Not real case data.",
        "features": FEATURES,
        "mean": [round(float(v), 6) for v in scaler.mean_],
        "scale": [round(float(v), 6) for v in scaler.scale_],
        "coef": [round(float(v), 6) for v in lr.coef_[0]],
        "intercept": round(float(lr.intercept_[0]), 6),
        "high_risk_threshold": THRESHOLD,
        "positive_rate": round(float(y.mean()), 4),
        "metrics_test_synthetic": {
            "logistic_regression": metrics(y_te, p_lr),
            "gradient_boosting_comparison": metrics(y_te, p_gb),
            "test_rows": int(len(y_te)),
        },
    }
    out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "internal", "ai", "risk", "model.json")
    with open(out, "w", encoding="utf-8") as f:
        json.dump(model, f, indent=2)
    print(json.dumps(model["metrics_test_synthetic"], indent=2))
    for name, c in sorted(zip(FEATURES, model["coef"]), key=lambda t: -abs(t[1])):
        print(f"{name:28s} {c:+.3f}")


if __name__ == "__main__":
    main()
