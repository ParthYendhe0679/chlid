package rules

import (
	"fmt"
	"strconv"
	"time"
)

// Age limits from section 3.2: [age group, max composite age for a couple, max age for a single parent, youngest child age].
var ageLimits = []struct {
	Group     string
	Couple    int
	Single    int
	ChildFrom int
}{
	{"0-2", 85, 40, 0},
	{"2-4", 90, 45, 2},
	{"4-8", 100, 50, 4},
	{"8-18", 110, 55, 8},
}

var AgeGroups = []string{"0-2", "2-4", "4-8", "8-18"}

type EligibilityInput struct {
	MaritalStatus    string   `json:"marital_status" validate:"required,oneof=MARRIED SINGLE_FEMALE SINGLE_MALE DIVORCED WIDOWED"`
	ApplicantDOB     string   `json:"applicant_dob" validate:"required"`
	SpouseDOB        string   `json:"spouse_dob"`
	YearsMarried     *float64 `json:"years_married" validate:"omitempty,min=0,max=80"`
	ExistingChildren int      `json:"existing_children" validate:"min=0,max=30"`
}

type Reason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type EligibilityResult struct {
	Eligible         bool     `json:"eligible"`
	AllowedAgeGroups []string `json:"allowed_age_groups"`
	Reasons          []Reason `json:"reasons"`
}

var (
	msgMarriage = Text{
		"A married couple must have been in a stable marriage for at least 2 years. You entered {years} years.",
		"विवाहित दंपति का विवाह कम से कम 2 साल पुराना होना चाहिए। आपने {years} साल लिखा है।",
		"विवाहित जोडप्याचे लग्न किमान 2 वर्षे टिकलेले असावे. तुम्ही {years} वर्षे लिहिले आहे.",
	}
	msgChildren = Text{
		"A couple with 3 or more children cannot adopt. The exceptions are a child with special needs, a hard-to-place child, a relative's child or a step-child.",
		"जिन दंपतियों के 3 या अधिक बच्चे हैं, वे गोद नहीं ले सकते। अपवाद: विशेष ज़रूरतों वाला बच्चा, कठिनाई से परिवार पाने वाला बच्चा, रिश्तेदार का बच्चा या सौतेला बच्चा।",
		"ज्या जोडप्यांना 3 किंवा अधिक मुले आहेत, ते दत्तक घेऊ शकत नाहीत. अपवाद: विशेष गरजा असलेले मूल, कुटुंब मिळण्यास कठीण असलेले मूल, नातेवाईकाचे मूल किंवा सावत्र मूल.",
	}
	msgCoupleAge = Text{
		"Your combined age is {age}. For children {group} years the limit is {limit}, so this age group is not allowed.",
		"आप दोनों की कुल आयु {age} है। {group} साल के बच्चों के लिए सीमा {limit} है, इसलिए यह आयु वर्ग नहीं मिलेगा।",
		"तुम्हा दोघांचे एकत्रित वय {age} आहे. {group} वर्षांच्या मुलांसाठी मर्यादा {limit} आहे, म्हणून हा वयोगट मिळणार नाही.",
	}
	msgSingleAge = Text{
		"Your age is {age}. For children {group} years the limit for a single parent is {limit}, so this age group is not allowed.",
		"आपकी आयु {age} है। {group} साल के बच्चों के लिए अकेले माता-पिता की सीमा {limit} है, इसलिए यह आयु वर्ग नहीं मिलेगा।",
		"तुमचे वय {age} आहे. {group} वर्षांच्या मुलांसाठी एकल पालकाची मर्यादा {limit} आहे, म्हणून हा वयोगट मिळणार नाही.",
	}
	msgAgeGap = Text{
		"There must be at least 25 years between the child and each parent, so children {group} years are not allowed.",
		"बच्चे और हर माता-पिता के बीच कम से कम 25 साल का अंतर होना चाहिए, इसलिए {group} साल के बच्चे नहीं मिलेंगे।",
		"मूल आणि प्रत्येक पालक यांच्या वयात किमान 25 वर्षांचे अंतर असावे, म्हणून {group} वर्षांची मुले मिळणार नाहीत.",
	}
	msgSingleMale = Text{
		"A single man cannot adopt a girl child. You can adopt a boy.",
		"अकेला पुरुष बालिका को गोद नहीं ले सकता। आप बालक को गोद ले सकते हैं।",
		"एकटा पुरुष मुलगी दत्तक घेऊ शकत नाही. तुम्ही मुलगा दत्तक घेऊ शकता.",
	}
	msgSingleFemale = Text{
		"A single woman can adopt a child of any gender.",
		"अकेली महिला किसी भी लिंग के बच्चे को गोद ले सकती है।",
		"एकटी महिला कोणत्याही लिंगाचे मूल दत्तक घेऊ शकते.",
	}
	msgSingleOther = Text{
		"Single-parent age limits apply to you. A single man cannot adopt a girl child.",
		"आप पर अकेले माता-पिता की आयु सीमा लागू होती है। अकेला पुरुष बालिका को गोद नहीं ले सकता।",
		"तुम्हाला एकल पालकाची वयोमर्यादा लागू होते. एकटा पुरुष मुलगी दत्तक घेऊ शकत नाही.",
	}
	msgCoupleConsent = Text{
		"Both partners must agree to the adoption.",
		"गोद लेने के लिए दोनों साथियों की सहमति ज़रूरी है।",
		"दत्तक घेण्यासाठी दोन्ही जोडीदारांची संमती आवश्यक आहे.",
	}
	msgFitness = Text{
		"You must also be physically, mentally and financially able to care for a child, with no life-threatening illness and no criminal conviction.",
		"आपको शारीरिक, मानसिक और आर्थिक रूप से बच्चे की देखभाल में सक्षम होना चाहिए, कोई जानलेवा बीमारी नहीं होनी चाहिए और कोई आपराधिक सज़ा नहीं होनी चाहिए।",
		"तुम्ही शारीरिक, मानसिक आणि आर्थिकदृष्ट्या मुलाची काळजी घेण्यास सक्षम असावे, कोणताही जीवघेणा आजार नसावा आणि कोणतीही गुन्हेगारी शिक्षा झालेली नसावी.",
	}
	msgNoGroup = Text{
		"No child age group is allowed with these details.",
		"इन जानकारियों के साथ कोई भी आयु वर्ग नहीं मिलता।",
		"या माहितीनुसार कोणताही वयोगट मिळत नाही.",
	}
)

// IST is used for all date arithmetic (the service runs in India).
var IST = func() *time.Location {
	l, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*3600+1800)
	}
	return l
}()

func ParseDate(s string) (time.Time, error) {
	if len(s) > 10 {
		s = s[:10]
	}
	return time.ParseInLocation("2006-01-02", s, IST)
}

// AgeYears returns completed years of age on the given day.
func AgeYears(dob, on time.Time) int {
	y := on.Year() - dob.Year()
	if on.Month() < dob.Month() || (on.Month() == dob.Month() && on.Day() < dob.Day()) {
		y--
	}
	return y
}

// CheckEligibility applies section 3.2.
func CheckEligibility(in EligibilityInput, lang string, now time.Time) (EligibilityResult, error) {
	res := EligibilityResult{Eligible: true, AllowedAgeGroups: []string{}, Reasons: []Reason{}}
	add := func(code string, t Text, args ...string) {
		res.Reasons = append(res.Reasons, Reason{code, t.In(lang, args...)})
	}

	dob, err := ParseDate(in.ApplicantDOB)
	if err != nil || !dob.Before(now) {
		return res, fmt.Errorf("applicant_dob must be a past date (YYYY-MM-DD)")
	}
	age := AgeYears(dob, now)
	married := in.MaritalStatus == "MARRIED"
	youngest := age
	composite := age
	if married {
		if in.SpouseDOB == "" {
			return res, fmt.Errorf("spouse_dob is required for a married couple")
		}
		sd, err := ParseDate(in.SpouseDOB)
		if err != nil || !sd.Before(now) {
			return res, fmt.Errorf("spouse_dob must be a past date (YYYY-MM-DD)")
		}
		sa := AgeYears(sd, now)
		composite = age + sa
		if sa < youngest {
			youngest = sa
		}
		years := 0.0
		if in.YearsMarried != nil {
			years = *in.YearsMarried
		}
		if years < 2 {
			res.Eligible = false
			add("MARRIAGE_DURATION", msgMarriage, "years", strconv.FormatFloat(years, 'f', -1, 64))
		}
		add("COUPLE_CONSENT", msgCoupleConsent)
	}
	if in.ExistingChildren >= 3 {
		res.Eligible = false
		add("CHILDREN_COUNT", msgChildren)
	}
	for _, l := range ageLimits {
		limit := l.Single
		if married {
			limit = l.Couple
		}
		if composite > limit {
			if married {
				add("COMPOSITE_AGE", msgCoupleAge, "age", strconv.Itoa(composite), "group", l.Group, "limit", strconv.Itoa(limit))
			} else {
				add("SINGLE_AGE", msgSingleAge, "age", strconv.Itoa(composite), "group", l.Group, "limit", strconv.Itoa(limit))
			}
			continue
		}
		if youngest-l.ChildFrom < 25 {
			add("AGE_GAP", msgAgeGap, "group", l.Group)
			continue
		}
		res.AllowedAgeGroups = append(res.AllowedAgeGroups, l.Group)
	}
	switch in.MaritalStatus {
	case "SINGLE_MALE":
		add("SINGLE_MALE_GIRL", msgSingleMale)
	case "SINGLE_FEMALE":
		add("SINGLE_FEMALE_ANY", msgSingleFemale)
	case "DIVORCED", "WIDOWED":
		add("SINGLE_RULES", msgSingleOther)
	}
	if len(res.AllowedAgeGroups) == 0 {
		res.Eligible = false
		add("NO_AGE_GROUP", msgNoGroup)
	}
	if !res.Eligible {
		res.AllowedAgeGroups = []string{}
	}
	add("FITNESS", msgFitness)
	return res, nil
}

// AgeText describes a child's age in simple words, e.g. "1 year 4 months".
func AgeText(dob, now time.Time, lang string) string {
	months := (now.Year()-dob.Year())*12 + int(now.Month()) - int(dob.Month())
	if now.Day() < dob.Day() {
		months--
	}
	if months < 0 {
		months = 0
	}
	y, m := months/12, months%12
	unit := func(n int, en1, enN, hi, mr string) string {
		switch lang {
		case "hi":
			return fmt.Sprintf("%d %s", n, hi)
		case "mr":
			return fmt.Sprintf("%d %s", n, mr)
		}
		if n == 1 {
			return fmt.Sprintf("%d %s", n, en1)
		}
		return fmt.Sprintf("%d %s", n, enN)
	}
	ys := unit(y, "year", "years", "साल", "वर्षे")
	ms := unit(m, "month", "months", "महीने", "महिने")
	switch {
	case y == 0:
		return ms
	case m == 0:
		return ys
	}
	return ys + " " + ms
}
