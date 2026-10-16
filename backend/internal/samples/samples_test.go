package samples_test

import (
	"bytes"
	"context"
	"image"
	"testing"
	"time"

	"childconnect/internal/ai"
	"childconnect/internal/samples"
	"childconnect/internal/storage"
)

var rahul = samples.Person{Name: "Rahul Sharma", Spouse: "Priya Sharma", DOB: "10-03-1988", Address: "B-12, Shanti Nagar, Mira Road, Thane 401107",
	City: "Thane", Employer: "Sahyadri Infotech Pvt Ltd", Salary: 95000, IDLast4: "4417", Seed: 7}

func check(t *testing.T, cat string, data []byte, others ...string) ai.DocResult {
	t.Helper()
	mime, ok := storage.SniffType(data)
	if !ok {
		t.Fatalf("%s: unknown type", cat)
	}
	svc := &ai.Service{}
	return svc.CheckDocument(context.Background(), ai.DocInput{Data: data, Mime: mime, Category: cat,
		ApplicantNames: ai.ApplicantNames("Rahul and Priya Sharma"), OtherAddresses: others, Now: time.Now()})
}

func codes(r ai.DocResult) map[string]string {
	m := map[string]string{}
	for _, c := range r.Checks {
		m[c.Code] = c.Result
	}
	return m
}

func TestSampleDocuments(t *testing.T) {
	cases := []struct {
		name, cat string
		data      []byte
		status    string
		mustHave  []string
	}{
		{"electricity bill passes", "RESIDENCE", samples.ElectricityBillPDF(rahul, time.Now().AddDate(0, 0, -20)), "AI_PASS", []string{"TYPE_ALLOWED", "NAME_MATCH", "BILL_RECENT"}},
		{"old bill warns", "RESIDENCE", samples.ElectricityBillPDF(rahul, time.Now().AddDate(0, -5, 0)), "AI_WARN", []string{"BILL_OLD"}},
		{"rent agreement warns", "RESIDENCE", samples.RentAgreementPDF(rahul), "AI_WARN", []string{"RESIDENCE_TYPE_NOT_IN_LIST"}},
		{"aadhaar identity passes", "IDENTITY", samples.AadhaarPDF(rahul), "AI_PASS", []string{"TYPE_ALLOWED", "NAME_MATCH"}},
		{"pan identity passes", "IDENTITY", samples.PANPDF(rahul), "AI_PASS", []string{"NAME_MATCH"}},
		{"birth proof passes", "BIRTH_PROOF", samples.BirthCertificatePDF(rahul), "AI_PASS", []string{"TYPE_ALLOWED"}},
		{"salary slip passes", "INCOME", samples.SalarySlipPDF(rahul, time.Now().AddDate(0, -1, 0)), "AI_PASS", []string{"TYPE_ALLOWED", "NAME_MATCH"}},
		{"medical without reg no warns", "MEDICAL", samples.MedicalCertificatePDF(rahul, false), "AI_WARN", []string{"DOCTOR_REG_MISSING"}},
		{"medical with reg no passes", "MEDICAL", samples.MedicalCertificatePDF(rahul, true), "AI_PASS", []string{"DOCTOR_REG_PRESENT"}},
		{"marriage passes", "MARRIAGE", samples.MarriageCertificatePDF(rahul), "AI_PASS", []string{"NAME_MATCH"}},
		{"reference passes", "REFERENCE", samples.ReferenceLetterPDF(rahul, "Sunita Kale"), "AI_PASS", []string{"TYPE_ALLOWED"}},
		{"bill in income category fails", "INCOME", samples.ElectricityBillPDF(rahul, time.Now()), "AI_FAIL", []string{"TYPE_NOT_ALLOWED"}},
		{"blurry photo fails", "PHOTO", samples.FamilyPhoto("Sharma family", true), "AI_FAIL", []string{"IMAGE_BLURRY"}},
	}
	for _, c := range cases {
		r := check(t, c.cat, c.data)
		got := codes(r)
		if r.Status != c.status {
			t.Errorf("%s: status %s, want %s (checks %v, type %v, fields %v)", c.name, r.Status, c.status, got, deref(r.DetectedType), r.Fields)
		}
		for _, m := range c.mustHave {
			if _, ok := got[m]; !ok {
				t.Errorf("%s: missing check %s (checks %v, fields %v)", c.name, m, got, r.Fields)
			}
		}
	}
	// Address on the bill must match the address on the Aadhaar card.
	r := check(t, "RESIDENCE", samples.ElectricityBillPDF(rahul, time.Now().AddDate(0, 0, -10)), rahul.Address)
	if codes(r)["ADDRESS_MATCH"] != "PASS" {
		t.Errorf("address match: %v", codes(r))
	}
}

func TestPhotoSharpness(t *testing.T) {
	for _, blurry := range []bool{false, true} {
		img, _, err := image.Decode(bytes.NewReader(samples.FamilyPhoto("x", blurry)))
		if err != nil {
			t.Fatal(err)
		}
		v := ai.LaplacianVariance(img)
		t.Logf("blurry=%v variance=%.1f", blurry, v)
		if blurry == (v >= ai.BlurThreshold) {
			t.Errorf("blurry=%v but variance %.1f vs threshold %.1f", blurry, v, ai.BlurThreshold)
		}
	}
	img, _, _ := image.Decode(bytes.NewReader(samples.AadhaarPNG(rahul)))
	t.Logf("id card variance=%.1f", ai.LaplacianVariance(img))
}

func TestNames(t *testing.T) {
	cands := ai.ApplicantNames("Rahul and Priya Sharma")
	for _, n := range []string{"Rahul Sharma", "Mr. Rahul K Sharma", "Rahul Kumar Sharma", "PRIYA SHARMA", "Smt Priya Sharma"} {
		if !ai.NameMatches(n, cands) {
			t.Errorf("%q should match %v", n, cands)
		}
	}
	for _, n := range []string{"Amit Verma", "Rohan Mehta", "Pooja Iyer"} {
		if ai.NameMatches(n, cands) {
			t.Errorf("%q should not match", n)
		}
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
