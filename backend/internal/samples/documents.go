package samples

import (
	"fmt"
	"image/color"
	"strings"
	"time"
)

// Person holds synthetic details printed on sample documents.
type Person struct {
	Name     string // e.g. "Rahul Sharma"
	Spouse   string // e.g. "Priya Sharma" ("" for single applicants)
	DOB      string // DD-MM-YYYY
	Address  string
	City     string
	Employer string
	Salary   int // monthly gross, rupees
	IDLast4  string
	Seed     int // varies numbers so every file is unique
}

const banner = "SAMPLE - SYNTHETIC DOCUMENT FOR CHILD CONNECT TESTING"

func AadhaarPDF(p Person) []byte {
	return PDF("Aadhaar card (sample)", []Line{
		H("Government of India"), B("Unique Identification Authority of India (UIDAI)"), T(banner),
		Gap("Aadhaar", 10), T("Name: " + p.Name), T("Date of Birth: " + p.DOB),
		T("Address: " + p.Address), T(fmt.Sprintf("Aadhaar No: XXXX XXXX %s", p.IDLast4)),
		Gap("Aadhaar - Mera Aadhaar, Meri Pehchaan", 10),
	})
}

func AadhaarPNG(p Person) []byte {
	return IDCard("GOVERNMENT OF INDIA - AADHAAR (SAMPLE)", []string{
		"Unique Identification Authority of India", "Name: " + p.Name, "DOB: " + p.DOB,
		"Address: " + p.Address, "Aadhaar No: XXXX XXXX " + p.IDLast4,
	}, color.RGBA{230, 120, 30, 255})
}

func PANPDF(p Person) []byte {
	return PDF("PAN card (sample)", []Line{
		H("Income Tax Department"), B("Government of India"), T(banner),
		Gap("Permanent Account Number Card", 10), T("Name: " + p.Name), T("Date of Birth: " + p.DOB),
		T(fmt.Sprintf("Permanent Account Number: ABCPS%04dK", 1000+p.Seed%9000)),
	})
}

func BirthCertificatePDF(p Person) []byte {
	return PDF("Birth certificate (sample)", []Line{
		H("Birth Certificate"), B("Issued under Section 12 of the Registration of Births and Deaths Act, 1969"), T(banner),
		Gap("Name: "+p.Name, 10), T("Date of Birth: " + p.DOB), T("Place of Birth: " + p.City),
		T(fmt.Sprintf("Registration No: B-%d/%04d", 1970+p.Seed%30, 100+p.Seed%900)),
		T("Date of Issue: 12-03-2015"), T("Registrar of Births and Deaths"),
	})
}

func ElectricityBillPDF(p Person, billDate time.Time) []byte {
	return PDF("Electricity bill (sample)", []Line{
		H("Maharashtra State Electricity Distribution Co. Ltd (MSEDCL)"), B("Electricity Bill - Residential"), T(banner),
		Gap("Consumer Name: "+p.Name, 10), T(fmt.Sprintf("Consumer No: %012d", 170000000000+int64(p.Seed)*7919)),
		T("Address: " + p.Address), T("Bill Date: " + billDate.Format("2006-01-02")),
		T(fmt.Sprintf("Units Consumed: %d kWh", 150+p.Seed%200)), T(fmt.Sprintf("Amount Payable: Rs %d", 1100+p.Seed%900)),
		T("Due Date: " + billDate.AddDate(0, 0, 18).Format("2006-01-02")),
	})
}

func RentAgreementPDF(p Person) []byte {
	return PDF("Rent agreement (sample)", []Line{
		H("Leave and Licence Agreement"), B("Rent agreement for residential premises"), T(banner),
		Gap("This agreement is made between the Licensor, Mr Arvind Mehta, and the Licensee:", 10),
		T("Name: " + p.Name), T("Address: " + p.Address),
		T("Monthly rent: Rs 18,000. Security deposit: Rs 60,000."), T("Period: 11 months from 01-04-2026."),
		T("Signed by the Licensor and the Licensee before two witnesses."),
	})
}

func SalarySlipPDF(p Person, month time.Time) []byte {
	gross := p.Salary
	return PDF("Salary slip (sample)", []Line{
		H(p.Employer), B("Salary Slip for " + month.Format("January 2006")), T(banner),
		Gap("Employee Name: "+p.Name, 10), T(fmt.Sprintf("Employee ID: EMP%05d", 10000+p.Seed)),
		T(fmt.Sprintf("Gross Salary: Rs %d", gross)), T(fmt.Sprintf("Deductions (PF, professional tax, TDS): Rs %d", gross/8)),
		T(fmt.Sprintf("Net Pay: Rs %d", gross-gross/8)), T("Date: " + month.AddDate(0, 1, -1).Format("2006-01-02")),
	})
}

func MedicalCertificatePDF(p Person, withRegNo bool) []byte {
	lines := []Line{
		H("Medical Certificate"), B("Certificate of fitness for prospective adoptive parent"), T(banner),
		Gap("Patient Name: "+p.Name, 10), T("Date of Birth: " + p.DOB),
		T("I have examined the above person. The person has no chronic, contagious or fatal disease"),
		T("and is medically fit to adopt and care for a child."),
		T("Date of Issue: " + time.Now().AddDate(0, 0, -20).Format("2006-01-02")),
		Gap("Dr. Kiran Bhosale, MBBS, MD", 16), T("Registered Medical Practitioner, " + p.City),
	}
	if withRegNo {
		lines = append(lines, T(fmt.Sprintf("Reg. No: MMC-%d-%05d", 2005+p.Seed%15, 1000+p.Seed%90000)))
	}
	return PDF("Medical certificate (sample)", lines)
}

func MarriageCertificatePDF(p Person) []byte {
	return PDF("Marriage certificate (sample)", []Line{
		H("Certificate of Marriage"), B("Marriage Certificate under the Maharashtra Regulation of Marriage Bureaus Act"), T(banner),
		Gap("Husband's Name: "+p.Name, 10), T("Wife's Name: " + p.Spouse),
		T("The marriage was solemnized on 14-02-2016 at " + p.City + "."),
		T(fmt.Sprintf("Registration No: MR/%d/%04d", 2016, 100+p.Seed%900)), T("Date of Issue: 20-03-2016"),
	})
}

func ReferenceLetterPDF(p Person, from string) []byte {
	return PDF("Reference letter (sample)", []Line{
		H("Reference Letter"), T(banner), Gap("To whom it may concern,", 10),
		T("I have known " + p.Name + " for more than ten years as a neighbour and family friend."),
		T("They are caring, responsible and respected in our community."),
		T("I strongly recommend them as adoptive parents."),
		Gap("Name: "+from, 14), T("Date: " + time.Now().AddDate(0, 0, -15).Format("2006-01-02")),
	})
}

// ChildStudyReportPDF and MedicalExaminationReportPDF are synthetic child reports, identified only by case code.
func ChildStudyReportPDF(caseCode, ageText, gender string) []byte {
	return PDF("Child Study Report (sample)", []Line{
		H("Child Study Report (CSR)"), B("Case code: " + caseCode), T(banner),
		Gap("Age: "+ageText+". Gender: "+strings.ToLower(gender)+".", 10),
		T("Background: admitted to the agency after being declared legally free by the Child Welfare Committee."),
		T("Personality: cheerful, responds well to caregivers, enjoys music and play."),
		T("Routine: sleeps well, eats home-cooked food, follows a daily routine."),
		T("Needs: a warm, patient family; regular paediatric check-ups."),
	})
}

func MedicalExaminationReportPDF(caseCode string, findings []string) []byte {
	lines := []Line{H("Medical Examination Report (MER)"), B("Case code: " + caseCode), T(banner), Gap("Findings:", 10)}
	for _, f := range findings {
		lines = append(lines, T("- "+f))
	}
	lines = append(lines, Gap("Examined by: Medical Officer, District Hospital (synthetic)", 12), T("Date: "+time.Now().AddDate(0, 0, -12).Format("2006-01-02")))
	return PDF("Medical Examination Report (sample)", lines)
}
