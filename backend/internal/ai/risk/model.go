// Package risk scores open deadlines with the logistic regression model trained offline (section 8.4).
// The weights come from model.json (ml/train_risk.py); Go computes the weighted sum and the sigmoid.
package risk

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

//go:embed model.json
var modelJSON []byte

type Model struct {
	Name      string         `json:"name"`
	Version   string         `json:"version"`
	Data      string         `json:"data"`
	Features  []string       `json:"features"`
	Mean      []float64      `json:"mean"`
	Scale     []float64      `json:"scale"`
	Coef      []float64      `json:"coef"`
	Intercept float64        `json:"intercept"`
	Threshold float64        `json:"high_risk_threshold"`
	Metrics   map[string]any `json:"metrics_test_synthetic"`
}

func Load() (*Model, error) {
	var m Model
	if err := json.Unmarshal(modelJSON, &m); err != nil {
		return nil, err
	}
	if len(m.Features) == 0 || len(m.Features) != len(m.Coef) || len(m.Mean) != len(m.Coef) || len(m.Scale) != len(m.Coef) {
		return nil, fmt.Errorf("model.json is inconsistent")
	}
	return &m, nil
}

// Features for one open deadline.
type Features struct {
	DaysElapsedRatio float64
	AgencyOpenCases  float64
	AgencyStaffCount float64
	AgencyPastLate   float64
	DocsReupload     float64
	Step             string
	ParentReplyDelay float64
	AvgLoadPerStaff  float64 // across all agencies, used only to explain workload
}

func (f Features) vector() map[string]float64 {
	oh := func(s string) float64 {
		if f.Step == s {
			return 1
		}
		return 0
	}
	return map[string]float64{
		"days_elapsed_ratio":      f.DaysElapsedRatio,
		"agency_open_cases":       f.AgencyOpenCases,
		"agency_staff_count":      f.AgencyStaffCount,
		"agency_past_late_rate":   f.AgencyPastLate,
		"docs_reupload_count":     f.DocsReupload,
		"step_HSR":                oh("HSR_60D"),
		"step_CSR_MER":            oh("CSR_MER_10D"),
		"step_MATCHING":           oh("MATCHING_30D"),
		"step_DM_FILING":          oh("DM_FILING_10D"),
		"step_DM_ORDER":           oh("DM_ORDER_60D"),
		"step_FOLLOWUP":           oh("FOLLOWUP_VISIT"),
		"parent_reply_delay_days": f.ParentReplyDelay,
	}
}

// Predict returns the probability of missing the deadline and the top 3 reasons in simple words.
func (m *Model) Predict(f Features) (float64, []string) {
	v := f.vector()
	z := m.Intercept
	type contrib struct {
		name string
		c    float64
	}
	var cs []contrib
	for i, name := range m.Features {
		std := (v[name] - m.Mean[i]) / m.Scale[i]
		c := m.Coef[i] * std
		z += c
		cs = append(cs, contrib{name, c})
	}
	p := 1 / (1 + math.Exp(-z))
	sort.Slice(cs, func(a, b int) bool { return cs[a].c > cs[b].c })
	var reasons []string
	for _, c := range cs {
		if len(reasons) == 3 || c.c <= 0.1 {
			break
		}
		if r := reason(c.name, f); r != "" {
			reasons = append(reasons, r)
		}
	}
	return math.Round(p*1000) / 1000, reasons
}

var stepWords = map[string]string{
	"HSR_60D": "Home study reports", "CSR_MER_10D": "Child reports", "MATCHING_30D": "Matching steps",
	"DM_FILING_10D": "District Magistrate filings", "DM_ORDER_60D": "District Magistrate orders", "FOLLOWUP_VISIT": "Follow-up visits",
}

func reason(name string, f Features) string {
	switch name {
	case "days_elapsed_ratio":
		return fmt.Sprintf("%d%% of the allowed time is already used", int(math.Round(f.DaysElapsedRatio*100)))
	case "agency_open_cases":
		if f.AgencyStaffCount > 0 && f.AvgLoadPerStaff > 0 {
			x := (f.AgencyOpenCases / f.AgencyStaffCount) / f.AvgLoadPerStaff
			if x >= 1.2 {
				return fmt.Sprintf("This agency has %.1f times the usual workload per staff member", x)
			}
		}
		return fmt.Sprintf("This agency has %d open cases", int(f.AgencyOpenCases))
	case "agency_staff_count":
		if f.AgencyStaffCount > 2 {
			return ""
		}
		return fmt.Sprintf("This agency has only %d staff members", int(f.AgencyStaffCount))
	case "agency_past_late_rate":
		return fmt.Sprintf("This agency missed %d%% of its past deadlines", int(math.Round(f.AgencyPastLate*100)))
	case "docs_reupload_count":
		if f.DocsReupload == 1 {
			return "Documents had to be uploaded again once"
		}
		return fmt.Sprintf("Documents had to be uploaded again %d times", int(f.DocsReupload))
	case "parent_reply_delay_days":
		return fmt.Sprintf("The parent takes about %.0f days to reply to messages", f.ParentReplyDelay)
	case "step_HSR", "step_CSR_MER", "step_MATCHING", "step_DM_FILING", "step_DM_ORDER", "step_FOLLOWUP":
		if w, ok := stepWords[f.Step]; ok {
			return w + " are often late across agencies"
		}
	}
	return ""
}
