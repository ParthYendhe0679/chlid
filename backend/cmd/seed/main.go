// Command seed loads the synthetic starting data (section 19): agencies, staff accounts, parent cases
// across every stage with legal deadlines, children (case codes only), the Sahayak knowledge base,
// and a folder of sample documents for live testing. All people and documents are synthetic.
//
//	go run ./cmd/seed -reset            wipe the database and load everything
//	go run ./cmd/seed -samples ../sample-documents
package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"

	"childconnect/internal/ai"
	"childconnect/internal/ai/risk"
	"childconnect/internal/audit"
	"childconnect/internal/auth"
	"childconnect/internal/config"
	"childconnect/internal/crypto"
	"childconnect/internal/db"
	"childconnect/internal/handlers"
	"childconnect/internal/rules"
	"childconnect/internal/samples"
	"childconnect/internal/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed knowledge_en.txt
var kbEN string

//go:embed knowledge_hi.txt
var kbHI string

//go:embed knowledge_mr.txt
var kbMR string

// Password shared by the presentation accounts. Change it with -password.
const defaultPassword = "ChildConnect@2026"

type seeder struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	box      *crypto.Box
	store    *storage.Local
	svc      *ai.Service
	rng      *rand.Rand
	now      time.Time
	pwHash   string
	agencies map[string]agencyInfo
	workers  map[string]uuid.UUID
	codes    map[string]int
}

type agencyInfo struct {
	ID       uuid.UUID
	Name     string
	State    string
	District string
	Speed    float64 // 1.0 = typical; above 1 = slower than the law allows more often
}

func main() {
	reset := flag.Bool("reset", false, "drop all tables and recreate them before seeding (deletes all data)")
	samplesDir := flag.String("samples", "../sample-documents", "folder to write sample documents for live testing ('' to skip)")
	password := flag.String("password", defaultPassword, "password for the presentation accounts")
	flag.Parse()
	log.SetFlags(0)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	if *reset {
		log.Println("resetting schema (all data will be removed)...")
		if err := db.Reset(cfg.DatabaseURLDirect); err != nil {
			log.Fatal(err)
		}
	} else if err := db.Migrate(cfg.DatabaseURLDirect); err != nil {
		log.Fatal(err)
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	if n > 0 {
		log.Fatal("the database already has data; run with -reset to replace it")
	}
	box, err := crypto.New(cfg.FieldKey)
	if err != nil {
		log.Fatal(err)
	}
	store, err := storage.NewLocal(cfg.StorageDir, cfg.JWTSecret, cfg.PublicAPIURL)
	if err != nil {
		log.Fatal(err)
	}
	svc := &ai.Service{Pool: pool}
	if cfg.AIEnabled() {
		if g, err := ai.NewGemini(ctx, cfg.GeminiAPIKey, cfg.GeminiModel, cfg.EmbedModel); err == nil {
			svc.Gem = g
		}
	}
	hash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatal(err)
	}
	s := &seeder{ctx: ctx, pool: pool, box: box, store: store, svc: svc, rng: rand.New(rand.NewSource(20261017)),
		now: time.Now().In(rules.IST), pwHash: hash, agencies: map[string]agencyInfo{}, workers: map[string]uuid.UUID{}, codes: map[string]int{}}

	must(s.seedAgenciesAndStaff())
	log.Println("agencies and staff: done")
	must(s.seedKnowledge())
	log.Println("knowledge base: done")
	must(s.seedChildrenFree())
	must(s.seedMainCases())
	log.Println("presentation cases: done")
	must(s.seedSyntheticCases())
	log.Println("synthetic cases: done")
	must(s.seedExtras())
	m, err := risk.Load()
	must(err)
	updated, err := m.Recompute(ctx, pool)
	must(err)
	log.Printf("risk scores: %d deadlines scored", updated)
	must(audit.New(pool).Log(ctx, audit.Entry{Action: "SYSTEM_SEED_LOADED", TargetType: "database", TargetID: "seed",
		Meta: map[string]any{"note": "synthetic data loaded", "embed_model": svc.EmbedModelName()}}))
	if *samplesDir != "" {
		must(writeSamples(*samplesDir, s.now))
		log.Printf("sample documents written to %s", *samplesDir)
	}
	fmt.Printf("\nSeed complete. Password for every account below: %s\n\n", *password)
	for _, l := range []string{
		"ADMIN   admin@childconnect.in       Kavita Rao, state oversight officer",
		"AGENCY  worker@childconnect.in      Meera Kulkarni, social worker, Asha Shishu Griha (Thane)",
		"AGENCY  doctor@childconnect.in      Dr. Sunil Patil, medical officer, Asha Shishu Griha",
		"AGENCY  nagpur@childconnect.in      Navjeevan Bal Sadan (agency waiting for admin approval)",
		"PARENT  parent@childconnect.in      Rahul and Priya Sharma: registered, 12 days left to upload documents",
		"PARENT  parent2@childconnect.in     Anita and Vikram Deshmukh: waiting list, highest seniority (receives the referral)",
		"PARENT  parent3@childconnect.in     Sanjay and Neha Joshi: documents submitted, ready for agency review",
		"PARENT  parent4@childconnect.in     Imran and Sana Shaikh: home study in progress (Report Co-Pilot)",
		"PARENT  parent5@childconnect.in     Arjun and Kavya Nair: adopted, follow-up visits running",
	} {
		fmt.Println("  " + l)
	}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func (s *seeder) days(n float64) time.Duration { return time.Duration(n * 24 * float64(time.Hour)) }

func (s *seeder) between(a, b float64) float64 { return a + s.rng.Float64()*(b-a) }

// ---------- agencies and staff ----------

func (s *seeder) seedAgenciesAndStaff() error {
	type ag struct {
		key, name, reg, district, phone string
		staff                           int
		approved                        bool
		speed                           float64
		createdAgo                      float64
	}
	list := []ag{
		{"thane", "Asha Shishu Griha", "MH-SAA-0123", "Thane", "022-2534-1180", 4, true, 1.0, 900},
		{"mumbai", "Mumbai Bal Ashray", "MH-SAA-0217", "Mumbai", "022-2414-6620", 6, true, 0.95, 1100},
		{"pune", "Pune Shishu Kendra", "MH-SAA-0456", "Pune", "020-2612-4471", 5, true, 0.75, 1000},
		{"nashik", "Nashik Bal Seva Sanstha", "MH-SAA-0311", "Nashik", "0253-257-3302", 2, true, 1.55, 950},
		{"kolhapur", "Kolhapur Bal Griha", "MH-SAA-0389", "Kolhapur", "0231-265-0915", 3, true, 1.2, 800},
		{"nagpur", "Navjeevan Bal Sadan", "MH-SAA-0502", "Nagpur", "0712-254-7781", 3, false, 1.0, 6},
	}
	for _, a := range list {
		var id uuid.UUID
		var reviewed any
		if a.approved {
			reviewed = s.now.Add(-s.days(a.createdAgo - 5))
		}
		err := s.pool.QueryRow(s.ctx, `INSERT INTO agencies (name, reg_number, agency_type, state, district, staff_count, phone, email, is_approved, reviewed_at, created_at)
			VALUES ($1,$2,'SAA','Maharashtra',$3,$4,$5,$6,$7,$8,$9) RETURNING id`, a.name, a.reg, a.district, a.staff, a.phone,
			"office."+a.key+"@childconnect.in", a.approved, reviewed, s.now.Add(-s.days(a.createdAgo))).Scan(&id)
		if err != nil {
			return err
		}
		s.agencies[a.key] = agencyInfo{ID: id, Name: a.name, State: "Maharashtra", District: a.district, Speed: a.speed}
	}
	staff := []struct {
		email, name, agency, designation string
	}{
		{"worker@childconnect.in", "Meera Kulkarni", "thane", "SOCIAL_WORKER"},
		{"doctor@childconnect.in", "Dr. Sunil Patil", "thane", "MEDICAL_OFFICER"},
		{"superintendent@childconnect.in", "Shobha Pawar", "thane", "SUPERINTENDENT"},
		{"mumbai@childconnect.in", "Farhan Qureshi", "mumbai", "SOCIAL_WORKER"},
		{"pune@childconnect.in", "Snehal Gokhale", "pune", "SOCIAL_WORKER"},
		{"nashik@childconnect.in", "Ravindra Jadhav", "nashik", "SOCIAL_WORKER"},
		{"kolhapur@childconnect.in", "Ashwini Mane", "kolhapur", "SOCIAL_WORKER"},
		{"nagpur@childconnect.in", "Prakash Wankhede", "nagpur", "SUPERINTENDENT"},
	}
	for _, st := range staff {
		var id uuid.UUID
		if err := s.pool.QueryRow(s.ctx, `INSERT INTO users (email, phone, password_hash, full_name, role, agency_id, designation, language, consent_at)
			VALUES ($1,$2,$3,$4,'AGENCY',$5,$6,'en',now()) RETURNING id`, st.email, "+91 98200 "+fmt.Sprintf("%05d", s.rng.Intn(99999)),
			s.pwHash, st.name, s.agencies[st.agency].ID, st.designation).Scan(&id); err != nil {
			return err
		}
		if st.designation == "SOCIAL_WORKER" {
			s.workers[st.agency] = id
		}
		if st.email == "doctor@childconnect.in" {
			s.workers["thane-doctor"] = id
		}
	}
	_, err := s.pool.Exec(s.ctx, `INSERT INTO users (email, phone, password_hash, full_name, role, language, consent_at)
		VALUES ('admin@childconnect.in', '+91 98190 44120', $1, 'Kavita Rao', 'ADMIN', 'en', now())`, s.pwHash)
	return err
}

// ---------- knowledge base ----------

var kbHeading = regexp.MustCompile(`^(Regulation|Schedule|Section|Fact|विनियम|धारा|तथ्य)(\s|$)`)

func (s *seeder) seedKnowledge() error {
	urls := map[string]string{
		"Adoption Regulations 2022":                                   "https://cara.wcd.gov.in/PDF/adoption%20regulations%202022%20english_27.pdf",
		"Juvenile Justice (Care and Protection of Children) Act 2015": "https://cara.wcd.gov.in/",
		"Child Connect verified facts (guide section 3)":              "https://cara.wcd.gov.in/",
	}
	for lang, text := range map[string]string{"en": kbEN, "hi": kbHI, "mr": kbMR} {
		var titles, cites, contents []string
		for _, block := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
			block = strings.TrimSpace(block)
			if block == "" {
				continue
			}
			for _, part := range handlers.SplitKnowledge(block) {
				head := part[0]
				title := "Child Connect verified facts (guide section 3)"
				switch {
				case strings.HasPrefix(head, "Regulation"), strings.HasPrefix(head, "Schedule"), strings.HasPrefix(head, "विनियम"):
					title = "Adoption Regulations 2022"
				case strings.HasPrefix(head, "Section"), strings.HasPrefix(head, "धारा"):
					title = "Juvenile Justice (Care and Protection of Children) Act 2015"
				}
				if !kbHeading.MatchString(head) {
					continue
				}
				titles, cites, contents = append(titles, title), append(cites, head), append(contents, part[1])
			}
		}
		texts := make([]string, len(contents))
		for i := range contents {
			texts[i] = titles[i] + ", " + cites[i] + ": " + contents[i]
		}
		vecs, model, err := s.svc.Embed(s.ctx, texts, "RETRIEVAL_DOCUMENT")
		if err != nil {
			log.Printf("Gemini embeddings failed (%v); using the offline embedder", err)
			vecs, model = make([][]float32, len(texts)), ai.LocalEmbedModel
			for i, t := range texts {
				vecs[i] = ai.LocalEmbed(t)
			}
		}
		for i := range contents {
			u := urls[titles[i]]
			if _, err := s.pool.Exec(s.ctx, `INSERT INTO knowledge_chunks (source_title, citation, source_url, language, content, embedding, embed_model)
				VALUES ($1,$2,$3,$4,$5,$6::vector,$7)`, titles[i], cites[i], u, lang, contents[i], ai.VectorLiteral(vecs[i]), model); err != nil {
				return err
			}
		}
		log.Printf("  %s: %d chunks (%s)", lang, len(contents), model)
	}
	return nil
}

// ---------- people ----------

var firstM = []string{"Amit", "Rohan", "Suresh", "Nikhil", "Prasad", "Kunal", "Manish", "Deepak", "Vivek", "Ajay", "Harish", "Sameer", "Gaurav", "Yogesh", "Tushar", "Mahesh", "Abhijit", "Rajesh", "Omkar", "Varun"}
var firstF = []string{"Sneha", "Pooja", "Swati", "Rupali", "Asha", "Madhuri", "Shweta", "Priyanka", "Kavita", "Deepa", "Manasi", "Ritu", "Smita", "Ketaki", "Anjali", "Vaishali", "Archana", "Gauri", "Nandini", "Shruti"}
var surnames = []string{"Patil", "Kale", "Gaikwad", "Bhosale", "Iyer", "Verma", "Mehta", "Kamat", "Dixit", "Shinde", "Pandit", "Rane", "Chavan", "Thakur", "Menon", "Reddy", "Bapat", "Sawant", "Kapoor", "More"}
var areas = map[string][]string{
	"thane":    {"Naupada, Thane West", "Vartak Nagar, Thane", "Kopri, Thane East", "Mira Road East, Thane"},
	"mumbai":   {"Andheri East, Mumbai", "Dadar West, Mumbai", "Chembur, Mumbai", "Borivali West, Mumbai"},
	"pune":     {"Kothrud, Pune", "Aundh, Pune", "Hadapsar, Pune", "Baner, Pune"},
	"nashik":   {"Gangapur Road, Nashik", "College Road, Nashik", "Panchavati, Nashik"},
	"kolhapur": {"Rajarampuri, Kolhapur", "Tarabai Park, Kolhapur", "Shivaji Peth, Kolhapur"},
}
var employers = []string{"Sahyadri Infotech Pvt Ltd", "Western Railway", "Deccan Pharma Ltd", "State Bank of India", "Konkan Logistics Pvt Ltd", "Zilla Parishad School", "Bharat Electronics Ltd", "Sunrise Hospital"}

type caseSpec struct {
	email, first, spouse, surname string
	agency                        string
	status                        string
	frac                          float64 // elapsed share of the current step's legal limit
	marital                       string
	applicantAge, spouseAge       int
	prefAge, prefGender           string
	openSpecial                   bool
	children                      int
	seniorityDaysAgo              float64 // WAITING: how long in the queue
	docOverrides                  map[string]docOverride
	skipDocs                      []string
	language                      string
}

type docOverride struct {
	data   []byte
	status string // "" = keep the DocCheck result
	note   string
}

func (c caseSpec) fullName() string {
	if c.spouse == "" {
		return c.first + " " + c.surname
	}
	return c.first + " and " + c.spouse + " " + c.surname
}

func (s *seeder) person(c caseSpec, seed int) samples.Person {
	area := areas[c.agency][seed%len(areas[c.agency])]
	p := samples.Person{
		Name: c.first + " " + c.surname, DOB: fmt.Sprintf("%02d-%02d-%d", 1+seed%27, 1+seed%12, s.now.Year()-c.applicantAge),
		Address: fmt.Sprintf("Flat %d, %s Co-op Society, %s %d", 100+seed%400, []string{"Shanti", "Sai", "Gokul", "Anand", "Om"}[seed%5], area, 400000+seed%9999),
		City:    s.agencies[c.agency].District, Employer: employers[seed%len(employers)], Salary: 45000 + (seed%12)*9000, IDLast4: fmt.Sprintf("%04d", 1000+(seed*7919)%9000), Seed: seed,
	}
	if c.spouse != "" {
		p.Spouse = c.spouse + " " + c.surname
	}
	return p
}

// docsFor builds one unique synthetic file per required category.
func (s *seeder) docsFor(c caseSpec, p samples.Person, at time.Time) map[string][]byte {
	out := map[string][]byte{
		"PHOTO":       samples.FamilyPhoto(c.surname+" family "+p.IDLast4, false),
		"IDENTITY":    samples.AadhaarPDF(p),
		"BIRTH_PROOF": samples.BirthCertificatePDF(p),
		"RESIDENCE":   samples.ElectricityBillPDF(p, at.AddDate(0, 0, -25)),
		"INCOME":      samples.SalarySlipPDF(p, at.AddDate(0, -1, 0)),
		"MEDICAL":     samples.MedicalCertificatePDF(p, true),
		"REFERENCE":   samples.ReferenceLetterPDF(p, firstF[p.Seed%len(firstF)]+" "+surnames[(p.Seed+3)%len(surnames)]),
	}
	if c.marital == "MARRIED" {
		out["MARRIAGE"] = samples.MarriageCertificatePDF(p)
	}
	return out
}

// ---------- case builder ----------

type timeline struct {
	R, S, H, F, RS, M, DF, O time.Time
}

// plan computes a realistic history ending "now" at the given status.
func (s *seeder) plan(c caseSpec) timeline {
	sp := s.agencies[c.agency].Speed
	dDoc := s.between(8, 27)
	dHSR := s.between(38, 70) * sp
	dDecide := s.between(0.4, 1.8)
	dMatch := s.between(12, 32) * sp
	dFile := s.between(4, 13) * sp
	dOrder := s.between(35, 80) * sp
	var startOfCurrent, limit float64
	acc := map[string]float64{}
	acc["S"] = dDoc
	acc["H"] = acc["S"] + dHSR
	wait := c.seniorityDaysAgo
	if wait == 0 {
		wait = s.between(120, 600)
	}
	acc["F"] = acc["H"] + wait
	acc["RS"] = acc["F"] + dDecide
	acc["M"] = acc["RS"] + dMatch
	acc["DF"] = acc["M"] + dFile
	acc["O"] = acc["DF"] + dOrder
	switch c.status {
	case "REGISTERED", "LAPSED":
		startOfCurrent, limit = 0, 30
	case "DOCS_SUBMITTED", "DOCS_CORRECTION":
		startOfCurrent, limit = acc["S"], 20
	case "HSR_IN_PROGRESS":
		startOfCurrent, limit = acc["S"], 60
	case "WAITING":
		startOfCurrent, limit = acc["H"], 1
		c.frac = wait
		acc["F"] = 0
	case "REFERRED":
		startOfCurrent, limit = acc["F"], 2
	case "RESERVED":
		startOfCurrent, limit = acc["RS"], 30
	case "MATCHED_FOSTER_CARE":
		startOfCurrent, limit = acc["M"], 10
	case "DM_ORDER_PENDING":
		startOfCurrent, limit = acc["DF"], 60
	case "ADOPTED_FOLLOWUP":
		startOfCurrent, limit = acc["O"], 1
	case "COMPLETED":
		startOfCurrent, limit = acc["O"], 1
	}
	R := s.now.Add(-s.days(startOfCurrent + c.frac*limit))
	at := func(k string) time.Time { return R.Add(s.days(acc[k])) }
	return timeline{R: R, S: at("S"), H: at("H"), F: at("F"), RS: at("RS"), M: at("M"), DF: at("DF"), O: at("O")}
}

var statusOrder = []string{"REGISTERED", "DOCS_SUBMITTED", "DOCS_CORRECTION", "HSR_IN_PROGRESS", "WAITING", "REFERRED", "RESERVED", "MATCHED_FOSTER_CARE", "DM_ORDER_PENDING", "ADOPTED_FOLLOWUP", "COMPLETED"}

func rank(st string) int {
	for i, x := range statusOrder {
		if x == st {
			return i
		}
	}
	return 0
}

type built struct {
	appID    uuid.UUID
	parentID uuid.UUID
	tl       timeline
}

func (s *seeder) buildCase(c caseSpec, seed int, child *uuid.UUID) (*built, error) {
	ag := s.agencies[c.agency]
	worker := s.workers[c.agency]
	tl := s.plan(c)
	p := s.person(c, seed)
	st := c.status
	r := rank(st)
	if st == "LAPSED" {
		r = 0
	}
	past := func(min string) bool { return r >= rank(min) }
	lang := c.language
	if lang == "" {
		lang = "en"
	}
	email := c.email
	if email == "" {
		email = fmt.Sprintf("family.%s.%d@childconnect.in", strings.ToLower(c.surname), seed)
	}
	b := &built{tl: tl}
	err := pgx.BeginFunc(s.ctx, s.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(s.ctx, `INSERT INTO users (email, phone, password_hash, full_name, role, language, consent_at, created_at)
			VALUES ($1,$2,$3,$4,'PARENT',$5,$6,$6) RETURNING id`, email, fmt.Sprintf("+91 9%09d", 100000000+seed*7351%899999999), s.pwHash,
			c.fullName(), lang, tl.R).Scan(&b.parentID); err != nil {
			return err
		}
		dob := s.now.AddDate(-c.applicantAge, -(seed % 11), -(seed % 27))
		var spouseDOB, years any
		if c.marital == "MARRIED" {
			spouseDOB = s.now.AddDate(-c.spouseAge, -(seed % 7), -(seed % 19)).Format("2006-01-02")
			years = 3 + seed%9
		}
		addr, _ := s.box.Encrypt(p.Address)
		var submitted, seniority, hsrValid any
		if past("DOCS_SUBMITTED") {
			submitted, seniority = tl.S, tl.S
		}
		if past("WAITING") {
			hsrValid = tl.H.AddDate(3, 0, 0).Format("2006-01-02")
		}
		used := 0
		if err := tx.QueryRow(s.ctx, `INSERT INTO applications (parent_id, agency_id, social_worker_id, status, marital_status, years_married, applicant_dob, spouse_dob,
			existing_children, annual_income_inr, state, district, address_enc, pref_age_group, pref_gender, pref_states, open_special_needs, open_siblings,
			registered_at, docs_submitted_at, seniority_date, hsr_valid_until, referrals_used, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'Maharashtra',$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$18,now()) RETURNING id`,
			b.parentID, ag.ID, worker, st, c.marital, years, dob.Format("2006-01-02"), spouseDOB, c.children, float64(p.Salary*12+seed*1000),
			ag.District, addr, c.prefAge, c.prefGender, []string{"Maharashtra"}, c.openSpecial, seed%3 == 0, tl.R, submitted, seniority, hsrValid, used).Scan(&b.appID); err != nil {
			return err
		}
		app := b.appID
		dl := func(step string, start, due time.Time, done *time.Time, childID *uuid.UUID) error {
			state := "OPEN"
			if done != nil {
				state = "DONE"
			}
			_, err := tx.Exec(s.ctx, `INSERT INTO case_deadlines (application_id, child_id, agency_id, step, started_at, due_at, completed_at, state)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, app, childID, ag.ID, step, start, due, done, state)
			return err
		}
		ptr := func(t time.Time) *time.Time { return &t }

		// Documents (DocCheck runs for real, offline rules mode).
		if st != "LAPSED" {
			files := s.docsFor(c, p, tl.S)
			for _, cat := range rules.RequiredCategories(c.marital, c.children) {
				data, ok := files[cat]
				if ov, has := c.docOverrides[cat]; has && ov.data != nil {
					data, ok = ov.data, true
				}
				skip := !ok
				for _, x := range c.skipDocs {
					if x == cat {
						skip = true
					}
				}
				if skip {
					continue
				}
				up := tl.R.Add(s.days(s.between(1, 7)))
				if past("DOCS_SUBMITTED") && up.After(tl.S) {
					up = tl.S.Add(-time.Hour)
				}
				if err := s.insertDoc(tx, app, c, cat, data, up, past("HSR_IN_PROGRESS"), worker, tl.S); err != nil {
					return err
				}
			}
		}

		// Deadlines and history.
		docDone := tl.S
		switch {
		case st == "LAPSED":
			if err := dl("DOC_UPLOAD_30D", tl.R, tl.R.Add(s.days(30)), ptr(tl.R.Add(s.days(30.5))), nil); err != nil {
				return err
			}
		case st == "REGISTERED":
			if err := dl("DOC_UPLOAD_30D", tl.R, tl.R.Add(s.days(30)), nil, nil); err != nil {
				return err
			}
		default:
			if err := dl("DOC_UPLOAD_30D", tl.R, tl.R.Add(s.days(30)), &docDone, nil); err != nil {
				return err
			}
		}
		if st == "DOCS_CORRECTION" {
			if err := dl("DOC_UPLOAD_30D", tl.S, tl.S.Add(s.days(30)), nil, nil); err != nil { // correction window shown to the parent
				return err
			}
		}
		if past("HSR_IN_PROGRESS") {
			var done *time.Time
			if past("WAITING") {
				done = ptr(tl.H)
			}
			if err := dl("HSR_60D", tl.S, tl.S.Add(s.days(60)), done, nil); err != nil {
				return err
			}
		}
		if past("WAITING") {
			if err := s.insertHSR(tx, app, worker, c, tl.H); err != nil {
				return err
			}
		}
		if past("REFERRED") && child != nil {
			dec, decided := "PENDING", (*time.Time)(nil)
			exp := tl.F.Add(48 * time.Hour)
			if past("RESERVED") {
				dec, decided = "RESERVED", ptr(tl.RS)
				if past("MATCHED_FOSTER_CARE") {
					dec = "ACCEPTED"
				}
			}
			if _, err := tx.Exec(s.ctx, `INSERT INTO referrals (application_id, child_id, referral_number, sent_by, starts_at, expires_at, decision, decided_at)
				VALUES ($1,$2,1,$3,$4,$5,$6,$7)`, app, *child, worker, tl.F, exp, dec, decided); err != nil {
				return err
			}
			var done *time.Time
			if past("RESERVED") {
				done = ptr(tl.RS)
			}
			if err := dl("RESERVE_48H", tl.F, exp, done, child); err != nil {
				return err
			}
		}
		if past("RESERVED") {
			var done *time.Time
			if past("MATCHED_FOSTER_CARE") {
				done = ptr(tl.M)
			}
			if err := dl("MATCHING_30D", tl.RS, tl.RS.Add(s.days(30)), done, child); err != nil {
				return err
			}
		}
		ev := func(t string, d time.Time, ref string) error {
			_, err := tx.Exec(s.ctx, `INSERT INTO legal_events (application_id, event_type, event_date, reference_no, recorded_by, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
				app, t, d.Format("2006-01-02"), ref, worker, d)
			return err
		}
		if past("MATCHED_FOSTER_CARE") {
			if err := ev("MATCHED", tl.M, fmt.Sprintf("AC/%d/%03d", tl.M.Year(), seed)); err != nil {
				return err
			}
			fc := tl.M.Add(s.days(s.between(2, 7)))
			fcDone := (*time.Time)(nil)
			if fc.Before(s.now) && (past("DM_ORDER_PENDING") || s.rng.Intn(2) == 0) {
				fcDone = ptr(fc)
				if err := ev("FOSTER_CARE", fc, ""); err != nil {
					return err
				}
			}
			if err := dl("FOSTER_CARE_10D", tl.M, tl.M.Add(s.days(10)), fcDone, child); err != nil {
				return err
			}
			var done *time.Time
			if past("DM_ORDER_PENDING") {
				done = ptr(tl.DF)
			}
			if err := dl("DM_FILING_10D", tl.M, tl.M.Add(s.days(10)), done, child); err != nil {
				return err
			}
		}
		if past("DM_ORDER_PENDING") {
			if err := ev("DM_FILED", tl.DF, fmt.Sprintf("DM/%s/ADP/%d/%03d", strings.ToUpper(ag.District[:3]), tl.DF.Year(), seed)); err != nil {
				return err
			}
			var done *time.Time
			if past("ADOPTED_FOLLOWUP") {
				done = ptr(tl.O)
			}
			if err := dl("DM_ORDER_60D", tl.DF, tl.DF.Add(s.days(60)), done, child); err != nil {
				return err
			}
		}
		if past("ADOPTED_FOLLOWUP") {
			if err := ev("DM_ORDER", tl.O, fmt.Sprintf("DMO/%d/%04d", tl.O.Year(), 100+seed)); err != nil {
				return err
			}
			if err := ev("BIRTH_CERT", tl.O.Add(s.days(4)), fmt.Sprintf("BC/%d/%05d", tl.O.Year(), 1000+seed)); err != nil {
				return err
			}
			for i := 1; i <= 4; i++ {
				due := tl.O.AddDate(0, 6*i, 0)
				var visited any
				var final, flags any = nil, []string{}
				var signed any
				var done *time.Time
				if due.Before(s.now) || st == "COMPLETED" {
					v := due.Add(s.days(s.between(-6, 12) * ag.Speed))
					if v.After(s.now) {
						v = s.now.Add(-s.days(1))
					}
					visited, signed, done = v.Format("2006-01-02"), worker, ptr(v)
					fr, _ := json.Marshal(map[string]string{"child_health": "Growing well; vaccinations are up to date.", "adjustment": "Settled into the family routine.",
						"schooling": "Attends nursery regularly.", "bonding": "Warm bond with both parents.", "concerns": ""})
					final = fr
				}
				if _, err := tx.Exec(s.ctx, `INSERT INTO followups (application_id, visit_number, due_date, visited_on, final_report, concern_flags, signed_by) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
					app, i, due.Format("2006-01-02"), visited, final, flags, signed); err != nil {
					return err
				}
				if err := dl("FOLLOWUP_VISIT", tl.O.AddDate(0, 6*(i-1), 0), due, done, child); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return b, err
}

func (s *seeder) insertDoc(tx pgx.Tx, app uuid.UUID, c caseSpec, cat string, data []byte, at time.Time, verified bool, worker uuid.UUID, reviewAt time.Time) error {
	mime, _ := storage.SniffType(data)
	res := s.svc.CheckDocument(s.ctx, ai.DocInput{Data: data, Mime: mime, Category: cat, ApplicantNames: ai.ApplicantNames(c.fullName()), Now: at})
	key, err := s.store.Save("docs", data, mime)
	if err != nil {
		return err
	}
	status := res.Status
	var reviewer, note, reviewed any
	if ov, ok := c.docOverrides[cat]; ok && ov.status != "" {
		status = ov.status
		if ov.note != "" {
			note = ov.note
		}
		if ov.status == "VERIFIED" || ov.status == "REUPLOAD_REQUESTED" {
			reviewer, reviewed = worker, reviewAt.Add(s.days(s.between(1, 4)))
		}
	} else if verified {
		status, reviewer, reviewed = "VERIFIED", worker, reviewAt.Add(s.days(s.between(1, 6)))
	}
	fields, _ := json.Marshal(res.Fields)
	checks, _ := json.Marshal(res.Checks)
	sum := sha256hex(data)
	ext := map[string]string{"application/pdf": "pdf", "image/jpeg": "jpg", "image/png": "png"}[mime]
	var docID uuid.UUID
	if err := tx.QueryRow(s.ctx, `INSERT INTO documents (application_id, category, storage_key, file_name, mime_type, size_bytes, sha256, ai_detected_type, ai_fields,
		ai_confidence, ai_checks, status, reviewer_id, reviewer_note, uploaded_at, reviewed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING id`,
		app, cat, key, strings.ToLower(cat)+"."+ext, mime, len(data), sum, res.DetectedType, fields, res.Confidence, checks, status, reviewer, note, at, reviewed).Scan(&docID); err != nil {
		return err
	}
	out, _ := json.Marshal(res)
	var hd any
	if status == "VERIFIED" {
		if res.Status == "AI_PASS" {
			hd = "ACCEPTED"
		} else {
			hd = "REJECTED"
		}
	}
	_, err = tx.Exec(s.ctx, `INSERT INTO ai_runs (feature, target_type, target_id, model, latency_ms, output, confidence, human_decision, created_at)
		VALUES ('DOCCHECK','document',$1,$2,$3,$4,$5,$6,$7)`, docID, res.Model, res.LatencyMs, out, res.Confidence, hd, at)
	return err
}

func (s *seeder) insertHSR(tx pgx.Tx, app, worker uuid.UUID, c caseSpec, signed time.Time) error {
	sections := map[string]string{
		"family_background":    fmt.Sprintf("%s live as a nuclear family. They have been married for several years and both partners consent to adoption.", c.fullName()),
		"home_environment":     "Two-bedroom flat in a safe residential society with a park and a school nearby. The home is clean and has space for a child.",
		"health":               "Both applicants report good health; medical certificates from a registered practitioner are on file.",
		"finances":             "Stable salaried income sufficient to meet the needs of a child; no outstanding loans reported.",
		"motivation_readiness": "Long-standing wish to adopt; attended pre-adoption counselling and have realistic expectations.",
		"parenting_plan":       "One parent will take leave for the first months; childcare support is planned afterwards.",
		"support_system":       "Grandparents and siblings live nearby and are supportive of the adoption.",
		"observations":         "The applicants were warm, open and cooperative during the visit.",
	}
	b, _ := json.Marshal(sections)
	visit := signed.Add(-s.days(s.between(3, 12)))
	_, err := tx.Exec(s.ctx, `INSERT INTO home_study_reports (application_id, social_worker_id, visit_date, raw_notes, final_sections, missing_items, completeness_score, recommendation, signed_at, created_at)
		VALUES ($1,$2,$3,$4,$5,'{}',100,'SUITABLE',$6,$7)`, app, worker, visit.Format("2006-01-02"), "Home visit notes recorded on paper.", b, signed, visit)
	return err
}

// ---------- children ----------

type childSpec struct {
	agency, name, gender, status string
	ageMonths                    int
	special                      bool
	lfaDaysAgo                   int // 0 = not legally free
	reports                      bool
	summary                      string
	admittedDaysAgo              int
	lastVisitDaysAgo             int  // 0 = a recent visit is recorded, unless noVisits
	noVisits                     bool // never visited by family (Child Review Alert)
	findings                     []string
}

func (s *seeder) addChild(cs childSpec) (uuid.UUID, string, error) {
	ag := s.agencies[cs.agency]
	prefix := handlers.CaseCodePrefix(ag.State, ag.District, s.now.Year())
	if cs.admittedDaysAgo > 300 {
		prefix = handlers.CaseCodePrefix(ag.State, ag.District, s.now.AddDate(0, 0, -cs.admittedDaysAgo).Year())
	}
	s.codes[prefix]++
	code := fmt.Sprintf("%s%04d", prefix, s.codes[prefix])
	dob := s.now.AddDate(0, -cs.ageMonths, -s.rng.Intn(20))
	nameEnc, _ := s.box.Encrypt(cs.name)
	var lfa, cwc, csrKey, merKey, merText, summary, approvedBy, approvedAt, lastVisit any
	if cs.lfaDaysAgo > 0 {
		lfa = s.now.AddDate(0, 0, -cs.lfaDaysAgo).Format("2006-01-02")
		cwc = fmt.Sprintf("CWC/%s/LFA/%d/%03d", strings.ToUpper(ag.District[:3]), s.now.Year(), 10+s.rng.Intn(80))
	}
	ageText := rules.AgeText(dob, s.now, "en")
	if cs.reports {
		k1, err := s.store.Save("children", samples.ChildStudyReportPDF(code, ageText, cs.gender), "application/pdf")
		if err != nil {
			return uuid.Nil, "", err
		}
		mer := samples.MedicalExaminationReportPDF(code, cs.findings)
		k2, err := s.store.Save("children", mer, "application/pdf")
		if err != nil {
			return uuid.Nil, "", err
		}
		csrKey, merKey = k1, k2
		if t, err := ai.PDFText(mer); err == nil {
			merText = strings.TrimSpace(t)
		}
	}
	if cs.summary != "" {
		summary, approvedBy, approvedAt = cs.summary, s.workers["thane-doctor"], s.now.AddDate(0, 0, -3)
	}
	switch {
	case cs.lastVisitDaysAgo > 0:
		lastVisit = s.now.AddDate(0, 0, -cs.lastVisitDaysAgo).Format("2006-01-02")
	case !cs.noVisits:
		lastVisit = s.now.AddDate(0, 0, -(20 + s.rng.Intn(130))).Format("2006-01-02") // regular family visits recorded
	}
	admitted := cs.admittedDaysAgo
	if admitted == 0 {
		admitted = cs.ageMonths*30 - 20
		if admitted < 10 {
			admitted = 10
		}
	}
	var id uuid.UUID
	err := s.pool.QueryRow(s.ctx, `INSERT INTO children (agency_id, case_code, name_enc, dob, gender, is_special_needs, admitted_on, last_family_visit_on, cwc_lfa_number, lfa_date,
		csr_storage_key, mer_storage_key, mer_text, summary_approved, summary_approved_by, summary_approved_at, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING id`, ag.ID, code, nameEnc, dob.Format("2006-01-02"), cs.gender, cs.special,
		s.now.AddDate(0, 0, -admitted).Format("2006-01-02"), lastVisit, cwc, lfa, csrKey, merKey, merText, summary, approvedBy, approvedAt, cs.status).Scan(&id)
	if err != nil {
		return uuid.Nil, "", err
	}
	if cs.lfaDaysAgo > 0 {
		start := s.now.AddDate(0, 0, -cs.lfaDaysAgo)
		due := start.AddDate(0, 0, 10)
		var done any
		state := "OPEN"
		if cs.reports {
			done, state = start.AddDate(0, 0, 3+s.rng.Intn(9)), "DONE"
		}
		if _, err := s.pool.Exec(s.ctx, `INSERT INTO case_deadlines (child_id, agency_id, step, started_at, due_at, completed_at, state) VALUES ($1,$2,'CSR_MER_10D',$3,$4,$5,$6)`,
			id, ag.ID, start, due, done, state); err != nil {
			return uuid.Nil, "", err
		}
	}
	return id, code, nil
}

const healthySummary = "The medical report describes a healthy child who is growing as expected for this age. Vaccinations listed in the report are up to date. " +
	"The doctor found no illness that needs treatment now.\n\nQuestions you can ask a doctor:\n- Which vaccinations are due next?\n- What should we watch for in the first months at home?"

func (s *seeder) seedChildrenFree() error {
	list := []childSpec{
		// Ready for referral; special needs; matches a waiting family open to special needs.
		{agency: "thane", name: "Synthetic Child A", gender: "MALE", status: "READY", ageMonths: 37, special: true, lfaDaysAgo: 40, reports: true,
			summary:  "The report describes a mild hearing difficulty in the left ear. Many children with this condition do well with a hearing check every few months and, if advised, a hearing aid.\n\nQuestions you can ask a doctor:\n- How often should hearing be tested?\n- Would speech therapy help?",
			findings: []string{"Weight and height within normal range for age", "Mild conductive hearing loss, left ear", "Immunisation up to date", "Developmental milestones appropriate"}},
		// Legally free with both reports: the live demo drafts and approves the summary, then sends the referral.
		{agency: "thane", name: "Synthetic Child B", gender: "MALE", status: "LEGALLY_FREE", ageMonths: 20, lfaDaysAgo: 6, reports: true,
			findings: []string{"Low birth weight at birth (2.1 kg); now gaining weight steadily", "Haemoglobin 10.4 g/dL: mild anaemia, iron supplements started", "Immunisation up to date for age", "Developmental milestones normal", "HIV, HBsAg and VDRL screening negative"}},
		// In care for 14 months without a family visit: Child Review Alert.
		{agency: "thane", name: "Synthetic Child C", gender: "FEMALE", status: "IN_CARE", ageMonths: 70, admittedDaysAgo: 430, noVisits: true},
		{agency: "mumbai", name: "Synthetic Child D", gender: "FEMALE", status: "READY", ageMonths: 74, lfaDaysAgo: 55, reports: true, summary: healthySummary,
			findings: []string{"Healthy child", "Immunisation up to date", "No abnormality detected"}},
		// Legally free 14 days ago, reports still missing: Child Review Alert.
		{agency: "kolhapur", name: "Synthetic Child E", gender: "MALE", status: "LEGALLY_FREE", ageMonths: 30, lfaDaysAgo: 14},
		{agency: "nashik", name: "Synthetic Child F", gender: "FEMALE", status: "IN_CARE", ageMonths: 110, admittedDaysAgo: 520, lastVisitDaysAgo: 400},
		{agency: "pune", name: "Synthetic Child G", gender: "FEMALE", status: "LEGALLY_FREE", ageMonths: 50, lfaDaysAgo: 4, reports: true,
			findings: []string{"Healthy child", "Mild iron deficiency", "Immunisation up to date"}},
	}
	for _, c := range list {
		if _, _, err := s.addChild(c); err != nil {
			return err
		}
	}
	return nil
}

// ---------- presentation cases ----------

func (s *seeder) seedMainCases() error {
	// parent@: registered 18 days ago (12 days left); three documents already uploaded.
	rahul := caseSpec{email: "parent@childconnect.in", first: "Rahul", spouse: "Priya", surname: "Sharma", agency: "thane", status: "REGISTERED", frac: 0.6,
		marital: "MARRIED", applicantAge: 38, spouseAge: 36, prefAge: "0-2", prefGender: "ANY", skipDocs: []string{"RESIDENCE", "INCOME", "MEDICAL", "MARRIAGE", "REFERENCE"}}
	rp := rahulPerson()
	rahul.docOverrides = map[string]docOverride{
		"PHOTO": {data: samples.FamilyPhoto("Sharma family", false)}, "IDENTITY": {data: samples.AadhaarPDF(rp)}, "BIRTH_PROOF": {data: samples.BirthCertificatePDF(rp)},
	}
	b, err := s.buildCase(rahul, 7, nil)
	if err != nil {
		return err
	}
	// The address in the profile matches the sample documents.
	addr, _ := s.box.Encrypt(rp.Address)
	if _, err := s.pool.Exec(s.ctx, `UPDATE applications SET address_enc=$2, applicant_dob='1988-03-10', spouse_dob='1990-07-22', years_married=9, annual_income_inr=1450000 WHERE id=$1`, b.appID, addr); err != nil {
		return err
	}
	s.message(b.appID, s.workers["thane"], "Welcome to Child Connect. I am your social worker. Please upload all documents within 30 days. Ask me here if anything is unclear.", b.tl.R.Add(26*time.Hour))
	s.notify(b.parentID, "DOC_DEADLINE", "Upload your documents", "Please upload all required documents within 30 days.", "/parent/documents")

	// parent2@: in the waiting list for over 3 years: highest seniority for a child aged 0-2.
	anita := caseSpec{email: "parent2@childconnect.in", first: "Vikram", spouse: "Anita", surname: "Deshmukh", agency: "thane", status: "WAITING",
		marital: "MARRIED", applicantAge: 41, spouseAge: 39, prefAge: "0-2", prefGender: "ANY", seniorityDaysAgo: 950}
	b2, err := s.buildCase(anita, 11, nil)
	if err != nil {
		return err
	}
	s.message(b2.appID, b2.parentID, "Namaste. Is there any update on our waiting list position?", s.now.AddDate(0, 0, -9))
	s.message(b2.appID, s.workers["thane"], "Your home study is valid and you are high in the seniority list. We will inform you as soon as a referral is made.", s.now.AddDate(0, 0, -8))

	// parent3@: documents submitted; the residence proof is a rent agreement (AI warning) for the review demo.
	joshi := caseSpec{email: "parent3@childconnect.in", first: "Sanjay", spouse: "Neha", surname: "Joshi", agency: "thane", status: "DOCS_SUBMITTED", frac: 0.15,
		marital: "MARRIED", applicantAge: 35, spouseAge: 33, prefAge: "0-2", prefGender: "FEMALE"}
	jp := s.person(joshi, 23)
	joshi.docOverrides = map[string]docOverride{"RESIDENCE": {data: samples.RentAgreementPDF(jp)}, "MEDICAL": {data: samples.MedicalCertificatePDF(jp, false)}}
	b3, err := s.buildCase(joshi, 23, nil)
	if err != nil {
		return err
	}
	s.message(b3.appID, b3.parentID, "We have submitted all documents. Our residence proof is a rent agreement because the flat is rented.", s.now.AddDate(0, 0, -2))

	// parent4@: home study in progress, 48 of 60 days used.
	shaikh := caseSpec{email: "parent4@childconnect.in", first: "Imran", spouse: "Sana", surname: "Shaikh", agency: "thane", status: "HSR_IN_PROGRESS", frac: 0.8,
		marital: "MARRIED", applicantAge: 40, spouseAge: 37, prefAge: "2-4", prefGender: "ANY", openSpecial: true}
	if _, err := s.buildCase(shaikh, 31, nil); err != nil {
		return err
	}

	// parent5@: adopted; follow-up visits running (first visit due soon).
	cid, _, err := s.addChild(childSpec{agency: "thane", name: "Synthetic Child H", gender: "FEMALE", status: "PLACED", ageMonths: 26, lfaDaysAgo: 330, reports: true,
		summary: healthySummary, findings: []string{"Healthy child", "Immunisation up to date"}})
	if err != nil {
		return err
	}
	nair := caseSpec{email: "parent5@childconnect.in", first: "Arjun", spouse: "Kavya", surname: "Nair", agency: "thane", status: "ADOPTED_FOLLOWUP", frac: 170,
		marital: "MARRIED", applicantAge: 36, spouseAge: 34, prefAge: "0-2", prefGender: "ANY", seniorityDaysAgo: 400}
	b5, err := s.buildCase(nair, 41, &cid)
	if err != nil {
		return err
	}
	s.notify(b5.parentID, "STATUS_ADOPTED_FOLLOWUP", "Adoption order received", "The adoption order is recorded. Four follow-up visits are scheduled.", "/parent")
	return nil
}

func rahulPerson() samples.Person {
	return samples.Person{Name: "Rahul Sharma", Spouse: "Priya Sharma", DOB: "10-03-1988", Address: "B-12, Shanti Nagar, Mira Road East, Thane 401107",
		City: "Thane", Employer: "Sahyadri Infotech Pvt Ltd", Salary: 120000, IDLast4: "4417", Seed: 7}
}

// ---------- synthetic cases for the radar and scorecard ----------

func (s *seeder) seedSyntheticCases() error {
	type row struct {
		agency, status string
		frac           float64
	}
	rowsList := []row{
		{"mumbai", "REGISTERED", 0.25}, {"pune", "REGISTERED", 0.55}, {"nashik", "REGISTERED", 0.9}, {"kolhapur", "REGISTERED", 0.97}, {"thane", "REGISTERED", 0.4},
		{"mumbai", "DOCS_SUBMITTED", 0.2}, {"nashik", "DOCS_SUBMITTED", 0.5}, {"pune", "DOCS_SUBMITTED", 0.1},
		{"kolhapur", "DOCS_CORRECTION", 0.3}, {"mumbai", "DOCS_CORRECTION", 0.15},
		{"nashik", "HSR_IN_PROGRESS", 1.35}, {"nashik", "HSR_IN_PROGRESS", 0.9}, {"kolhapur", "HSR_IN_PROGRESS", 1.12}, {"mumbai", "HSR_IN_PROGRESS", 0.76},
		{"pune", "HSR_IN_PROGRESS", 0.35}, {"thane", "HSR_IN_PROGRESS", 0.55}, {"mumbai", "HSR_IN_PROGRESS", 0.2}, {"kolhapur", "HSR_IN_PROGRESS", 0.68},
		{"mumbai", "WAITING", 0}, {"pune", "WAITING", 0}, {"nashik", "WAITING", 0}, {"kolhapur", "WAITING", 0}, {"thane", "WAITING", 0}, {"pune", "WAITING", 0}, {"mumbai", "WAITING", 0},
		{"mumbai", "REFERRED", 0.4},
		{"pune", "RESERVED", 0.35}, {"nashik", "RESERVED", 0.85},
		{"kolhapur", "MATCHED_FOSTER_CARE", 0.8},
		{"nashik", "DM_ORDER_PENDING", 1.25}, {"mumbai", "DM_ORDER_PENDING", 0.5}, {"kolhapur", "DM_ORDER_PENDING", 0.9},
		{"pune", "ADOPTED_FOLLOWUP", 260}, {"mumbai", "ADOPTED_FOLLOWUP", 90},
		{"pune", "COMPLETED", 760},
		{"kolhapur", "LAPSED", 1.2},
	}
	prefFor := []struct {
		age, gender string
		special     bool
	}{{"2-4", "ANY", true}, {"4-8", "FEMALE", false}, {"8-18", "ANY", true}, {"2-4", "MALE", false}, {"4-8", "ANY", false}, {"0-2", "FEMALE", false}, {"2-4", "ANY", false}}
	for i, r := range rowsList {
		seed := 100 + i*13
		c := caseSpec{first: firstM[i%len(firstM)], spouse: firstF[(i*3)%len(firstF)], surname: surnames[(i*7)%len(surnames)], agency: r.agency, status: r.status, frac: r.frac,
			marital: "MARRIED", applicantAge: 34 + i%9, spouseAge: 31 + i%8, prefAge: "0-2", prefGender: "ANY", children: 0}
		if i%9 == 4 {
			c.marital, c.spouse, c.first = "SINGLE_FEMALE", "", firstF[(i*5)%len(firstF)]
			c.applicantAge = 38
			c.prefAge = "2-4"
		}
		if r.status == "WAITING" {
			pf := prefFor[i%len(prefFor)]
			c.prefAge, c.prefGender, c.openSpecial = pf.age, pf.gender, pf.special
			c.seniorityDaysAgo = s.between(60, 800)
			if c.prefAge == "0-2" && c.seniorityDaysAgo > 900 {
				c.seniorityDaysAgo = 700
			}
			if c.prefAge == "4-8" || c.prefAge == "8-18" {
				c.applicantAge, c.spouseAge = 44+i%5, 42+i%4
			}
		}
		if r.status == "DOCS_CORRECTION" {
			p := s.person(c, seed)
			c.docOverrides = map[string]docOverride{"RESIDENCE": {data: samples.ElectricityBillPDF(p, s.now.AddDate(0, -6, 0)), status: "REUPLOAD_REQUESTED",
				note: "The electricity bill is older than 3 months. Please upload a bill from the last 3 months."}}
		}
		var child *uuid.UUID
		if rank(r.status) >= rank("REFERRED") && r.status != "LAPSED" {
			status := map[string]string{"REFERRED": "REFERRED", "RESERVED": "RESERVED"}[r.status]
			if status == "" {
				status = "PLACED"
			}
			ages := map[string]int{"0-2": 16, "2-4": 34, "4-8": 70, "8-18": 120}
			id, _, err := s.addChild(childSpec{agency: r.agency, name: fmt.Sprintf("Synthetic Child %d", i), gender: []string{"MALE", "FEMALE"}[i%2], status: status,
				ageMonths: ages[c.prefAge], lfaDaysAgo: 200 + i*3, reports: true, summary: healthySummary, findings: []string{"Healthy child", "Immunisation up to date"}})
			if err != nil {
				return err
			}
			child = &id
		}
		if _, err := s.buildCase(c, seed, child); err != nil {
			return fmt.Errorf("case %d (%s): %w", i, r.status, err)
		}
	}
	return nil
}

// ---------- extras ----------

func (s *seeder) seedExtras() error {
	scam := "Healthy newborn baby boy available for adoption, no paperwork needed, contact on WhatsApp 98XXXXXX21, Rs 50,000 advance."
	reasons, _ := json.Marshal([]string{"Offers a child directly, outside the legal process.", "Promises adoption without paperwork.", "Asks for an advance payment.", "Asks you to contact a person privately (for example on WhatsApp)."})
	if _, err := s.pool.Exec(s.ctx, `INSERT INTO scam_reports (text, verdict, reasons, created_at) VALUES ($1,'LIKELY_SCAM',$2,$3)`, scam, reasons, s.now.AddDate(0, 0, -3)); err != nil {
		return err
	}
	reasons2, _ := json.Marshal([]string{"Promises a quick adoption or skipping the waiting list.", "Asks for an advance payment."})
	_, err := s.pool.Exec(s.ctx, `INSERT INTO scam_reports (text, verdict, reasons, reviewed, created_at) VALUES ($1,'LIKELY_SCAM',$2,true,$3)`,
		"Adopt a baby girl within 2 weeks through our trust, skip the CARA waiting list. Processing fee Rs 2 lakh to be paid first.", reasons2, s.now.AddDate(0, 0, -11))
	return err
}

func sha256hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (s *seeder) message(app, sender uuid.UUID, body string, at time.Time) {
	_, _ = s.pool.Exec(s.ctx, `INSERT INTO messages (application_id, sender_id, body, created_at) VALUES ($1,$2,$3,$4)`, app, sender, body, at)
}

func (s *seeder) notify(user uuid.UUID, typ, title, body, link string) {
	_, _ = s.pool.Exec(s.ctx, `INSERT INTO notifications (user_id, type, title, body, link) VALUES ($1,$2,$3,$4,$5)`, user, typ, title, body, link)
}

// ---------- sample documents for live testing ----------

func writeSamples(dir string, now time.Time) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	p := rahulPerson()
	files := map[string][]byte{
		"01_family_photo.jpg":                                       samples.FamilyPhoto("Sharma family", false),
		"02_aadhaar_card.pdf":                                       samples.AadhaarPDF(p),
		"03_aadhaar_card_scan.png":                                  samples.AadhaarPNG(p),
		"04_birth_certificate.pdf":                                  samples.BirthCertificatePDF(p),
		"05_rent_agreement_(expect_warning).pdf":                    samples.RentAgreementPDF(p),
		"06_electricity_bill.pdf":                                   samples.ElectricityBillPDF(p, now.AddDate(0, 0, -21)),
		"07_salary_slip.pdf":                                        samples.SalarySlipPDF(p, now.AddDate(0, -1, 0)),
		"08_medical_certificate_no_reg_number_(expect_warning).pdf": samples.MedicalCertificatePDF(p, false),
		"09_medical_certificate.pdf":                                samples.MedicalCertificatePDF(p, true),
		"10_marriage_certificate.pdf":                               samples.MarriageCertificatePDF(p),
		"11_reference_letter.pdf":                                   samples.ReferenceLetterPDF(p, "Sunita Kale"),
		"12_blurry_photo_(expect_fail).jpg":                         samples.FamilyPhoto("Sharma family", true),
		"13_old_electricity_bill_(expect_warning).pdf":              samples.ElectricityBillPDF(p, now.AddDate(0, -5, 0)),
		"child_study_report_CSR.pdf":                                samples.ChildStudyReportPDF("(assigned by Child Connect)", "1 year 2 months", "FEMALE"),
		"medical_examination_report_MER.pdf": samples.MedicalExaminationReportPDF("(assigned by Child Connect)", []string{
			"Born preterm at 34 weeks; low birth weight (1.9 kg); now gaining weight steadily", "Haemoglobin 10.1 g/dL: mild anaemia; iron supplements advised",
			"Soft heart murmur heard; echocardiogram shows a small VSD that may close on its own", "Immunisation up to date", "HIV, HBsAg and VDRL negative"}),
	}
	texts := map[string]string{
		"hsr_rough_notes.txt": "Visited Imran (40) and Sana (37) Shaikh at their 2BHK flat in Naupada, Thane. Married 11 years, no children. " +
			"House is clean, separate room ready for child, park and school within walking distance. Imran works as an accountant, salary about 14 lakh per annum, own flat, no loans. " +
			"Sana is a school teacher, plans to take one year leave after placement. Both report good health, no chronic illness. " +
			"They attended counselling twice, motivation is long wish to have a family, open to a child up to 4 years and mild special needs. " +
			"Observed couple warm and cooperative, realistic expectations about adjustment.",
		"followup_visit_notes.txt": "Visit at Nair family home. Child is healthy, weight gain good, vaccinations up to date. Settled well into routine, sleeps through the night. " +
			"Started playschool last month, teacher says she plays with others. Strong bond with both parents, calls Kavya amma. " +
			"Parents mention she is not eating well for the last week and had mild fever, doctor visit done.",
		"scam_message.txt": "Healthy newborn baby boy available for adoption, no paperwork needed, contact on WhatsApp 98XXXXXX21, Rs 50,000 advance.",
		"README.txt": "SAMPLE documents for testing Child Connect. All are synthetic and marked SAMPLE. Not real people.\n\n" +
			"Parent upload (log in as parent@childconnect.in):\n" +
			"  PHOTO       01_family_photo.jpg  (12_blurry_photo shows the blur check failing)\n" +
			"  IDENTITY    02_aadhaar_card.pdf  (03_aadhaar_card_scan.png is read only when Gemini AI is enabled)\n" +
			"  BIRTH_PROOF 04_birth_certificate.pdf\n" +
			"  RESIDENCE   05_rent_agreement (warning), then 06_electricity_bill (passes); 13_old_electricity_bill shows the 3-month rule\n" +
			"  INCOME      07_salary_slip.pdf\n" +
			"  MEDICAL     08 (warning: no registration number), then 09_medical_certificate.pdf (passes)\n" +
			"  MARRIAGE    10_marriage_certificate.pdf\n" +
			"  REFERENCE   11_reference_letter.pdf\n\n" +
			"Agency: child_study_report_CSR.pdf and medical_examination_report_MER.pdf for a new child; hsr_rough_notes.txt for the Report Co-Pilot (case of Imran and Sana Shaikh);\n" +
			"followup_visit_notes.txt for the follow-up report (case of Arjun and Kavya Nair).\nPublic: scam_message.txt for Scam Shield.\n",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	for name, t := range texts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(t), 0o644); err != nil {
			return err
		}
	}
	return nil
}
