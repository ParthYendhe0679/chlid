package workflow

import (
	"context"
	"time"

	"childconnect/internal/rules"

	"github.com/google/uuid"
)

type Stage struct {
	Key   string  `json:"key"`
	Title string  `json:"title"`
	Done  bool    `json:"done"`
	Date  *string `json:"date"`
}

type Tracker struct {
	Status          string    `json:"status"`
	StageIndex      int       `json:"stage_index"`
	Stages          []Stage   `json:"stages"`
	CurrentDeadline *Deadline `json:"current_deadline"`
	NextStepText    string    `json:"next_step_text"`
	ReferralsUsed   int       `json:"referrals_used"`
}

var stageDefs = []struct {
	Key   string
	Title rules.Text
}{
	{"REGISTRATION", rules.Text{"Registration", "पंजीकरण", "नोंदणी"}},
	{"DOCUMENTS", rules.Text{"Document upload", "दस्तावेज़ अपलोड", "दस्तऐवज अपलोड"}},
	{"REVIEW", rules.Text{"Document review by agency", "एजेंसी द्वारा दस्तावेज़ जाँच", "संस्थेकडून दस्तऐवज तपासणी"}},
	{"HOME_STUDY", rules.Text{"Home Study Report", "गृह अध्ययन रिपोर्ट", "गृह अभ्यास अहवाल"}},
	{"WAITING", rules.Text{"Waiting list", "प्रतीक्षा सूची", "प्रतीक्षा यादी"}},
	{"REFERRAL", rules.Text{"Referral and reservation", "रेफ़रल और आरक्षण", "संदर्भ आणि आरक्षण"}},
	{"MATCHING", rules.Text{"Matching and foster care", "मिलान और पालन-पोषण देखभाल", "जुळणी आणि पालकत्व काळजी"}},
	{"ORDER", rules.Text{"Adoption order", "गोद लेने का आदेश", "दत्तक आदेश"}},
	{"FOLLOWUP", rules.Text{"Follow-up visits", "फ़ॉलो-अप विज़िट", "पाठपुरावा भेटी"}},
}

// stageOf maps a status to the stage that is in progress.
var stageOf = map[string]int{
	"REGISTERED": 1, "DOCS_CORRECTION": 1, "LAPSED": 1, "DOCS_SUBMITTED": 2, "HSR_IN_PROGRESS": 3,
	"WAITING": 4, "DEBARRED": 4, "REFERRED": 5, "RESERVED": 6, "MATCHED_FOSTER_CARE": 6,
	"DM_ORDER_PENDING": 7, "ADOPTED_FOLLOWUP": 8, "COMPLETED": 9,
}

var nextStep = map[string]rules.Text{
	"REGISTERED": {"Upload all required documents before the 30-day limit. DocCheck AI checks each one right away.",
		"30 दिन की सीमा से पहले सभी ज़रूरी दस्तावेज़ अपलोड करें। DocCheck AI हर दस्तावेज़ तुरंत जाँचता है।",
		"30 दिवसांच्या मर्यादेपूर्वी सर्व आवश्यक दस्तऐवज अपलोड करा. DocCheck AI प्रत्येक दस्तऐवज लगेच तपासतो."},
	"DOCS_SUBMITTED": {"Your social worker is reviewing your documents. You will be told if anything must be uploaded again.",
		"आपके सामाजिक कार्यकर्ता आपके दस्तावेज़ जाँच रहे हैं। अगर कुछ दोबारा अपलोड करना हो तो आपको बताया जाएगा।",
		"तुमचे सामाजिक कार्यकर्ते तुमचे दस्तऐवज तपासत आहेत. काही पुन्हा अपलोड करायचे असल्यास तुम्हाला कळवले जाईल."},
	"DOCS_CORRECTION": {"Your social worker asked you to upload a document again. Open Documents to see the reason.",
		"आपके सामाजिक कार्यकर्ता ने एक दस्तावेज़ दोबारा अपलोड करने को कहा है। कारण देखने के लिए दस्तावेज़ खोलें।",
		"तुमच्या सामाजिक कार्यकर्त्यांनी एक दस्तऐवज पुन्हा अपलोड करण्यास सांगितले आहे. कारण पाहण्यासाठी दस्तऐवज उघडा."},
	"HSR_IN_PROGRESS": {"Your social worker will visit your home and write the Home Study Report. The law allows 60 days.",
		"आपके सामाजिक कार्यकर्ता आपके घर आकर गृह अध्ययन रिपोर्ट लिखेंगे। कानून 60 दिन देता है।",
		"तुमचे सामाजिक कार्यकर्ते तुमच्या घरी भेट देऊन गृह अभ्यास अहवाल लिहितील. कायद्याने 60 दिवस दिले आहेत."},
	"WAITING": {"You are in the waiting list. Your place depends on your seniority date and preferences. You will be told when a referral arrives.",
		"आप प्रतीक्षा सूची में हैं। आपका स्थान आपकी वरिष्ठता तिथि और पसंद पर निर्भर है। रेफ़रल आने पर आपको बताया जाएगा।",
		"तुम्ही प्रतीक्षा यादीत आहात. तुमचे स्थान तुमची ज्येष्ठता तारीख आणि पसंतीवर अवलंबून आहे. संदर्भ आल्यावर तुम्हाला कळवले जाईल."},
	"REFERRED": {"A child's profile has been referred to you. Read the reports and decide within 48 hours.",
		"एक बच्चे की प्रोफ़ाइल आपको भेजी गई है। रिपोर्ट पढ़ें और 48 घंटे के भीतर निर्णय लें।",
		"एका मुलाची माहिती तुम्हाला पाठवली आहे. अहवाल वाचा आणि 48 तासांत निर्णय घ्या."},
	"RESERVED": {"You will meet the child and the agency's Adoption Committee will approve the match within 30 days.",
		"आप बच्चे से मिलेंगे और एजेंसी की दत्तक समिति 30 दिनों के भीतर मिलान को मंज़ूरी देगी।",
		"तुम्ही मुलाला भेटाल आणि संस्थेची दत्तक समिती 30 दिवसांत जुळणीला मंजुरी देईल."},
	"MATCHED_FOSTER_CARE": {"The child will live with you in pre-adoption foster care. The agency files the application with the District Magistrate within 10 days.",
		"बच्चा पूर्व-दत्तक पालन-पोषण देखभाल में आपके साथ रहेगा। एजेंसी 10 दिनों के भीतर ज़िला मजिस्ट्रेट के पास आवेदन दाखिल करेगी।",
		"मूल दत्तकपूर्व पालकत्व काळजीत तुमच्यासोबत राहील. संस्था 10 दिवसांत जिल्हाधिकाऱ्यांकडे अर्ज दाखल करेल."},
	"DM_ORDER_PENDING": {"The District Magistrate decides the application within 2 months (60 days) of filing.",
		"ज़िला मजिस्ट्रेट आवेदन दाखिल होने के 2 महीने (60 दिन) के भीतर निर्णय देते हैं।",
		"जिल्हाधिकारी अर्ज दाखल झाल्यापासून 2 महिन्यांत (60 दिवस) निर्णय देतात."},
	"ADOPTED_FOLLOWUP": {"Congratulations. A social worker will visit every 6 months for 2 years (4 visits).",
		"बधाई हो। एक सामाजिक कार्यकर्ता 2 साल तक हर 6 महीने में मिलने आएंगे (4 विज़िट)।",
		"अभिनंदन. एक सामाजिक कार्यकर्ता 2 वर्षे दर 6 महिन्यांनी भेट देतील (4 भेटी)."},
	"COMPLETED": {"All follow-up visits are complete. Your case is closed.",
		"सभी फ़ॉलो-अप विज़िट पूरी हो गई हैं। आपका मामला पूरा हुआ।",
		"सर्व पाठपुरावा भेटी पूर्ण झाल्या आहेत. तुमचे प्रकरण पूर्ण झाले आहे."},
	"LAPSED": {"Documents were not submitted within 30 days, so the registration lapsed. You need to register again.",
		"30 दिनों में दस्तावेज़ जमा नहीं हुए, इसलिए पंजीकरण समाप्त हो गया। आपको फिर से पंजीकरण करना होगा।",
		"30 दिवसांत दस्तऐवज जमा झाले नाहीत, म्हणून नोंदणी रद्द झाली. तुम्हाला पुन्हा नोंदणी करावी लागेल."},
	"DEBARRED": {"No child was reserved in 3 referrals, so you cannot apply again for 1 year (Regulation 9(3)).",
		"3 रेफ़रल में कोई बच्चा आरक्षित नहीं किया गया, इसलिए आप 1 साल तक फिर आवेदन नहीं कर सकते (विनियम 9(3))।",
		"3 संदर्भांमध्ये एकही मूल आरक्षित केले नाही, म्हणून तुम्ही 1 वर्ष पुन्हा अर्ज करू शकत नाही (विनियम 9(3))."},
	"WITHDRAWN": {"This application is closed.", "यह आवेदन बंद है।", "हा अर्ज बंद झाला आहे."},
}

// BuildTracker assembles the Case Tracker for one application.
func BuildTracker(ctx context.Context, q Querier, appID uuid.UUID, lang string, now time.Time) (*Tracker, error) {
	var status string
	var registered time.Time
	var submitted *time.Time
	var used int
	err := q.QueryRow(ctx, `SELECT status::text, registered_at, docs_submitted_at, referrals_used FROM applications WHERE id=$1`, appID).
		Scan(&status, &registered, &submitted, &used)
	if err != nil {
		return nil, err
	}
	dates := make([]*time.Time, len(stageDefs))
	dates[0] = &registered
	dates[1] = submitted
	var signed, reserved *time.Time
	_ = q.QueryRow(ctx, `SELECT signed_at FROM home_study_reports WHERE application_id=$1`, appID).Scan(&signed)
	_ = q.QueryRow(ctx, `SELECT decided_at FROM referrals WHERE application_id=$1 AND decision IN ('RESERVED','ACCEPTED') ORDER BY decided_at DESC LIMIT 1`, appID).Scan(&reserved)
	dates[2] = nil
	if st := stageOf[status]; st > 2 {
		var hsrStart *time.Time
		_ = q.QueryRow(ctx, `SELECT started_at FROM case_deadlines WHERE application_id=$1 AND step='HSR_60D' ORDER BY started_at LIMIT 1`, appID).Scan(&hsrStart)
		dates[2] = hsrStart
	}
	dates[3] = signed
	dates[5] = reserved
	events := map[string]time.Time{}
	rows, err := q.Query(ctx, `SELECT event_type, event_date FROM legal_events WHERE application_id=$1`, appID)
	if err == nil {
		for rows.Next() {
			var et string
			var d time.Time
			if rows.Scan(&et, &d) == nil {
				events[et] = d
			}
		}
		rows.Close()
	}
	if d, ok := events["MATCHED"]; ok {
		dates[6] = &d
	}
	if d, ok := events["DM_ORDER"]; ok {
		dates[7] = &d
	}
	var lastVisit *time.Time
	_ = q.QueryRow(ctx, `SELECT max(visited_on) FROM followups WHERE application_id=$1 AND visit_number=4`, appID).Scan(&lastVisit)
	dates[8] = lastVisit
	// The waiting stage ends when the first referral arrives.
	var firstRef *time.Time
	_ = q.QueryRow(ctx, `SELECT min(starts_at) FROM referrals WHERE application_id=$1`, appID).Scan(&firstRef)
	dates[4] = firstRef

	idx := stageOf[status]
	t := &Tracker{Status: status, StageIndex: idx, ReferralsUsed: used, NextStepText: nextStep[status].In(lang)}
	if idx >= len(stageDefs) {
		t.StageIndex = len(stageDefs) - 1
	}
	for i, sd := range stageDefs {
		s := Stage{Key: sd.Key, Title: sd.Title.In(lang), Done: i < idx}
		if s.Done && dates[i] != nil {
			v := dates[i].In(rules.IST).Format("2006-01-02")
			s.Date = &v
		}
		t.Stages = append(t.Stages, s)
	}
	cd, err := Current(ctx, q, appID, now)
	if err != nil {
		return nil, err
	}
	t.CurrentDeadline = cd
	return t, nil
}
