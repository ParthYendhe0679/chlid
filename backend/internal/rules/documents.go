package rules

import (
	"strings"
)

// Categories in the order the checklist shows them (section 3.6).
var AllCategories = []string{"PHOTO", "IDENTITY", "BIRTH_PROOF", "RESIDENCE", "INCOME", "MEDICAL", "MARRIAGE", "DIVORCE_OR_DEATH", "REFERENCE", "CHILD_CONSENT"}

// RequiredCategories depends on the profile: marriage proof for couples, divorce or death proof for
// divorced or widowed applicants, and consent of older children when the family already has children.
func RequiredCategories(marital string, existingChildren int) []string {
	out := []string{"PHOTO", "IDENTITY", "BIRTH_PROOF", "RESIDENCE", "INCOME", "MEDICAL"}
	switch marital {
	case "MARRIED":
		out = append(out, "MARRIAGE")
	case "DIVORCED", "WIDOWED":
		out = append(out, "DIVORCE_OR_DEATH")
	}
	out = append(out, "REFERENCE")
	if existingChildren > 0 {
		out = append(out, "CHILD_CONSENT")
	}
	return out
}

// DetectedTypes is the closed list the document reader may return.
var DetectedTypes = []string{
	"FAMILY_PHOTO", "PASSPORT_PHOTO", "PAN_CARD", "AADHAAR_CARD", "PASSPORT", "VOTER_ID", "DRIVING_LICENCE",
	"BIRTH_CERTIFICATE", "SCHOOL_LEAVING_CERTIFICATE", "ELECTRICITY_BILL", "TELEPHONE_BILL", "RENT_AGREEMENT",
	"SALARY_SLIP", "INCOME_CERTIFICATE", "INCOME_TAX_RETURN", "MEDICAL_CERTIFICATE", "MARRIAGE_CERTIFICATE",
	"DIVORCE_DECREE", "DEATH_CERTIFICATE", "REFERENCE_LETTER", "CONSENT_LETTER", "OTHER",
}

// AllowedTypes per category, from the Schedule VI list in section 3.6.
var AllowedTypes = map[string][]string{
	"PHOTO":            {"FAMILY_PHOTO", "PASSPORT_PHOTO"},
	"IDENTITY":         {"PAN_CARD", "AADHAAR_CARD", "PASSPORT", "VOTER_ID"},
	"BIRTH_PROOF":      {"BIRTH_CERTIFICATE", "SCHOOL_LEAVING_CERTIFICATE", "PASSPORT", "PAN_CARD", "AADHAAR_CARD", "DRIVING_LICENCE"},
	"RESIDENCE":        {"AADHAAR_CARD", "VOTER_ID", "PASSPORT", "ELECTRICITY_BILL", "TELEPHONE_BILL"},
	"INCOME":           {"SALARY_SLIP", "INCOME_CERTIFICATE", "INCOME_TAX_RETURN"},
	"MEDICAL":          {"MEDICAL_CERTIFICATE"},
	"MARRIAGE":         {"MARRIAGE_CERTIFICATE"},
	"DIVORCE_OR_DEATH": {"DIVORCE_DECREE", "DEATH_CERTIFICATE"},
	"REFERENCE":        {"REFERENCE_LETTER"},
	"CHILD_CONSENT":    {"CONSENT_LETTER"},
}

func TypeAllowed(category, detected string) bool {
	for _, t := range AllowedTypes[category] {
		if t == detected {
			return true
		}
	}
	return false
}

func HumanType(t string) string { return strings.ReplaceAll(strings.ToLower(t), "_", " ") }

func AllowedList(category string) string {
	var parts []string
	for _, t := range AllowedTypes[category] {
		parts = append(parts, HumanType(t))
	}
	return strings.Join(parts, ", ")
}

// NameCheckApplies says which categories carry the applicant's name.
func NameCheckApplies(category string) bool {
	switch category {
	case "IDENTITY", "BIRTH_PROOF", "RESIDENCE", "INCOME", "MEDICAL", "MARRIAGE", "DIVORCE_OR_DEATH":
		return true
	}
	return false
}

// Check messages for DocCheck results.
var CheckText = map[string]Text{
	"DUPLICATE_FILE": {"This exact file was already used in another application.", "यही फ़ाइल पहले से किसी दूसरे आवेदन में इस्तेमाल हो चुकी है।", "हीच फाइल आधीच दुसऱ्या अर्जात वापरली गेली आहे."},
	"IMAGE_RESOLUTION_LOW": {"The image is too small to read clearly ({w} x {h} pixels). Please upload a clearer copy.",
		"तस्वीर बहुत छोटी है और साफ़ नहीं पढ़ी जा सकती ({w} x {h} पिक्सेल)। कृपया साफ़ कॉपी अपलोड करें।",
		"प्रतिमा खूप लहान आहे आणि स्पष्ट वाचता येत नाही ({w} x {h} पिक्सेल). कृपया स्पष्ट प्रत अपलोड करा."},
	"IMAGE_BLURRY":     {"The image looks blurry. Please upload a clearer copy.", "तस्वीर धुंधली लग रही है। कृपया साफ़ कॉपी अपलोड करें।", "प्रतिमा अस्पष्ट दिसते. कृपया स्पष्ट प्रत अपलोड करा."},
	"IMAGE_QUALITY_OK": {"The image is sharp enough to read.", "तस्वीर पढ़ने लायक साफ़ है।", "प्रतिमा वाचण्याइतकी स्पष्ट आहे."},
	"NOT_READABLE":     {"The document could not be read. Please upload a clearer copy.", "दस्तावेज़ पढ़ा नहीं जा सका। कृपया साफ़ कॉपी अपलोड करें।", "दस्तऐवज वाचता आला नाही. कृपया स्पष्ट प्रत अपलोड करा."},
	"TYPE_ALLOWED":     {"This {type} is an accepted document for this category.", "यह {type} इस श्रेणी के लिए स्वीकार्य दस्तावेज़ है।", "हे {type} या प्रकारासाठी स्वीकार्य दस्तऐवज आहे."},
	"TYPE_NOT_ALLOWED": {"This looks like a {type}, which is not accepted for this category. Please upload one of: {list}.",
		"यह {type} लगता है, जो इस श्रेणी के लिए स्वीकार्य नहीं है। कृपया इनमें से कोई एक अपलोड करें: {list}।",
		"हे {type} असल्याचे दिसते, जे या प्रकारासाठी स्वीकार्य नाही. कृपया यापैकी एक अपलोड करा: {list}."},
	"RESIDENCE_TYPE_NOT_IN_LIST": {"A rent agreement is not in the usual list of accepted residence proofs. Please upload an accepted document, or your social worker will review this one.",
		"किराया अनुबंध (रेंट एग्रीमेंट) निवास प्रमाण की सामान्य सूची में नहीं है। कृपया कोई स्वीकार्य दस्तावेज़ अपलोड करें, या आपके सामाजिक कार्यकर्ता इसी दस्तावेज़ की जाँच करेंगे।",
		"भाडे करार (रेंट ॲग्रीमेंट) निवासाच्या पुराव्यांच्या नेहमीच्या यादीत नाही. कृपया स्वीकार्य दस्तऐवज अपलोड करा, किंवा तुमचे सामाजिक कार्यकर्ते हाच दस्तऐवज तपासतील."},
	"TYPE_UNKNOWN":       {"The document type could not be identified. Your social worker will review it.", "दस्तावेज़ का प्रकार पहचाना नहीं जा सका। आपके सामाजिक कार्यकर्ता इसकी जाँच करेंगे।", "दस्तऐवजाचा प्रकार ओळखता आला नाही. तुमचे सामाजिक कार्यकर्ते तो तपासतील."},
	"NAME_MATCH":         {"The name matches your application.", "नाम आपके आवेदन से मेल खाता है।", "नाव तुमच्या अर्जाशी जुळते."},
	"NAME_MISMATCH":      {"The name on the document ({found}) does not match your application.", "दस्तावेज़ पर लिखा नाम ({found}) आपके आवेदन से मेल नहीं खाता।", "दस्तऐवजावरील नाव ({found}) तुमच्या अर्जाशी जुळत नाही."},
	"NAME_NOT_FOUND":     {"No name could be read on the document.", "दस्तावेज़ पर कोई नाम पढ़ा नहीं जा सका।", "दस्तऐवजावर कोणतेही नाव वाचता आले नाही."},
	"ADDRESS_MATCH":      {"The address matches your other address document.", "पता आपके दूसरे पते वाले दस्तावेज़ से मेल खाता है।", "पत्ता तुमच्या दुसऱ्या पत्त्याच्या दस्तऐवजाशी जुळतो."},
	"ADDRESS_MISMATCH":   {"The address does not match the address on your other identity or residence document.", "पता आपके दूसरे पहचान या निवास दस्तावेज़ के पते से मेल नहीं खाता।", "पत्ता तुमच्या दुसऱ्या ओळख किंवा निवास दस्तऐवजावरील पत्त्याशी जुळत नाही."},
	"BILL_OLD":           {"This bill is older than 3 months ({date}). Please upload a recent bill.", "यह बिल 3 महीने से पुराना है ({date})। कृपया हाल का बिल अपलोड करें।", "हे बिल 3 महिन्यांपेक्षा जुने आहे ({date}). कृपया अलीकडचे बिल अपलोड करा."},
	"BILL_RECENT":        {"The bill is from the last 3 months.", "बिल पिछले 3 महीनों का है।", "बिल मागील 3 महिन्यांतील आहे."},
	"BILL_DATE_MISSING":  {"The bill date could not be read. Your social worker will check that it is recent.", "बिल की तारीख पढ़ी नहीं जा सकी। आपके सामाजिक कार्यकर्ता जाँचेंगे कि यह हाल का है।", "बिलाची तारीख वाचता आली नाही. तुमचे सामाजिक कार्यकर्ते ते अलीकडचे आहे का ते तपासतील."},
	"DOCTOR_REG_MISSING": {"The medical certificate does not show the doctor's registration number.", "मेडिकल प्रमाणपत्र पर डॉक्टर का पंजीकरण नंबर नहीं दिख रहा है।", "वैद्यकीय प्रमाणपत्रावर डॉक्टरांचा नोंदणी क्रमांक दिसत नाही."},
	"DOCTOR_REG_PRESENT": {"The doctor's registration number is present.", "डॉक्टर का पंजीकरण नंबर मौजूद है।", "डॉक्टरांचा नोंदणी क्रमांक आहे."},
	"LOW_CONFIDENCE": {"The automatic check is not fully sure about this document, so a person will review it.",
		"स्वचालित जाँच इस दस्तावेज़ के बारे में पूरी तरह निश्चित नहीं है, इसलिए एक व्यक्ति इसकी जाँच करेगा।",
		"स्वयंचलित तपासणीला या दस्तऐवजाबद्दल पूर्ण खात्री नाही, म्हणून एक व्यक्ती तो तपासेल."},
	"AI_CHECK_UNAVAILABLE": {"The automatic document reader could not run right now. Your social worker will review this document.",
		"स्वचालित दस्तावेज़ जाँच अभी नहीं चल सकी। आपके सामाजिक कार्यकर्ता इस दस्तावेज़ की जाँच करेंगे।",
		"स्वयंचलित दस्तऐवज तपासणी आत्ता चालू शकली नाही. तुमचे सामाजिक कार्यकर्ते हा दस्तऐवज तपासतील."},
	"IMAGE_NOT_READ_OFFLINE": {"Photos and scans are read by the AI document reader, which is switched off on this server. Your social worker will review this document.",
		"फ़ोटो और स्कैन AI दस्तावेज़ रीडर से पढ़े जाते हैं, जो इस सर्वर पर बंद है। आपके सामाजिक कार्यकर्ता इस दस्तावेज़ की जाँच करेंगे।",
		"फोटो आणि स्कॅन AI दस्तऐवज वाचकाद्वारे वाचले जातात, जो या सर्व्हरवर बंद आहे. तुमचे सामाजिक कार्यकर्ते हा दस्तऐवज तपासतील."},
	"PHOTO_RECEIVED": {"The photograph was received.", "फ़ोटो मिल गई है।", "फोटो मिळाला आहे."},
}
