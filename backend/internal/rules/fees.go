package rules

type FeeItem struct {
	Name   string `json:"name"`
	Amount int    `json:"amount"`
}

type Fees struct {
	Items            []FeeItem `json:"items"`
	TotalSingleChild int       `json:"total_single_child"`
	Notes            []string  `json:"notes"`
}

// OfficialFees returns the 2022 fee schedule from section 3.5.
func OfficialFees(lang string) Fees {
	names := map[string][4]string{
		"en": {"Home Study Report", "Adoption fee", "Follow-up visits (4 x 2000)", "Each additional sibling"},
		"hi": {"गृह अध्ययन रिपोर्ट", "गोद लेने का शुल्क", "फ़ॉलो-अप विज़िट (4 x 2000)", "हर अतिरिक्त भाई-बहन"},
		"mr": {"गृह अभ्यास अहवाल", "दत्तक शुल्क", "पाठपुरावा भेटी (4 x 2000)", "प्रत्येक अतिरिक्त भावंड"},
	}
	notes := map[string][]string{
		"en": {"Fees are non-refundable (CARA direction, January 2024).", "No extra payment is required. Any request for extra money to speed up an adoption is a warning sign of fraud.", "Fees are paid to the agency, never to an individual.", "Each additional sibling adds Rs 10,000 to the adoption fee."},
		"hi": {"शुल्क वापस नहीं होता (CARA निर्देश, जनवरी 2024)।", "कोई अतिरिक्त भुगतान ज़रूरी नहीं है। गोद लेने की प्रक्रिया तेज़ करने के लिए अतिरिक्त पैसे माँगना धोखाधड़ी का संकेत है।", "शुल्क एजेंसी को दिया जाता है, किसी व्यक्ति को नहीं।", "हर अतिरिक्त भाई-बहन के लिए गोद लेने के शुल्क में Rs 10,000 जुड़ते हैं।"},
		"mr": {"शुल्क परत मिळत नाही (CARA निर्देश, जानेवारी 2024).", "कोणतेही अतिरिक्त पैसे भरावे लागत नाहीत. दत्तक प्रक्रिया लवकर करण्यासाठी जास्त पैसे मागणे हे फसवणुकीचे लक्षण आहे.", "शुल्क संस्थेला दिले जाते, कोणत्याही व्यक्तीला नाही.", "प्रत्येक अतिरिक्त भावंडासाठी दत्तक शुल्कात Rs 10,000 वाढतात."},
	}
	if !ValidLang(lang) {
		lang = "en"
	}
	n := names[lang]
	return Fees{
		Items:            []FeeItem{{n[0], 6000}, {n[1], 50000}, {n[2], 8000}},
		TotalSingleChild: 64000,
		Notes:            notes[lang],
	}
}
