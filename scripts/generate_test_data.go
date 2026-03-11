package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"time"
)

var enNames = []string{"Alice Smith", "Bob Jones", "Charlie Brown", "David Wilson", "Eva Hansen"}
var daNames = []string{"Mette Jensen", "Lars Nielsen", "Hanne Pedersen", "Kristian Poulsen", "Sofie Andersen"}

var enEmails = []string{"alice@example.com", "bob.j@gmail.com", "david.wilson@company.com"}
var daEmails = []string{"mette.j@firma.dk", "lars.n@mail.dk", "hanne.p@tjeneste.dk"}

func generateCPR() string {
	day := rand.Intn(28) + 1
	month := rand.Intn(12) + 1
	year := rand.Intn(99)
	suffix := rand.Intn(9999)
	return fmt.Sprintf("%02d%02d%02d-%04d", day, month, year, suffix)
}

func generateOpenAIKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 48)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return "sk-" + string(b)
}

func main() {
	lang := flag.String("lang", "en", "Language for synthetic data (en or da)")
	outPath := flag.String("out", "", "Output file path (optional, defaults to stdout)")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	var out io.Writer = os.Stdout
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer f.Close()
		out = f
	}

	names := enNames
	emails := enEmails
	title := "--- Synthetic Test Data for NobodyProx ---"
	subjectPrefix := "Subject: Support Request #"
	fromPrefix := "From: "
	emailBody1 := "Hi support,\n\nI am %s, and I am having trouble with my account.\n"
	emailBody2 := "My registered email is %s and my Danish CPR for verification is %s.\n"
	emailBody3 := "Please help me reset my password.\n\nBest regards,\n"
	logMsg := "User accessed secure resource"
	codeComment := "// Application Code Snippets"
	secretComment := "// Hardcoded secret for testing"
	endMsg := "--- End of Test Data ---"

	if *lang == "da" {
		names = daNames
		emails = daEmails
		title = "--- Syntetiske Testdata for NobodyProx ---"
		subjectPrefix = "Emne: Supportanmodning #"
		fromPrefix = "Fra: "
		emailBody1 = "Hej support,\n\nJeg er %s, og jeg har problemer med min konto.\n"
		emailBody2 = "Min registrerede e-mail er %s og mit danske CPR-nummer til verifikation er %s.\n"
		emailBody3 = "Hjælp mig venligst med at nulstille min adgangskode.\n\nMed venlig hilsen,\n"
		logMsg = "Bruger fik adgang til sikker ressource"
		codeComment = "// Applikationskodestumper"
		secretComment = "// Hardcoded hemmelighed til test"
		endMsg = "--- Slut på testdata ---"
	}

	fmt.Fprintln(out, title)
	fmt.Fprintln(out)

	// 1. Support Emails
	for i := 0; i < 3; i++ {
		name := names[rand.Intn(len(names))]
		email := emails[rand.Intn(len(emails))]
		cpr := generateCPR()
		fmt.Fprintf(out, "%s%d\n", subjectPrefix, 1000+i)
		fmt.Fprintf(out, "%s%s <%s>\n", fromPrefix, name, email)
		fmt.Fprintf(out, emailBody1, name)
		fmt.Fprintf(out, emailBody2, email, cpr)
		fmt.Fprintln(out, emailBody3+name)
		fmt.Fprintln(out, "---")
	}

	// 2. JSON Logs
	fmt.Fprintln(out, "\n// System Logs (JSON format)")
	for i := 0; i < 5; i++ {
		name := names[rand.Intn(len(names))]
		cpr := generateCPR()
		key := generateOpenAIKey()
		fmt.Fprintf(out, "{\"timestamp\": \"%s\", \"level\": \"INFO\", \"user\": \"%s\", \"cpr\": \"%s\", \"api_key\": \"%s\", \"msg\": \"%s\"}\n", 
			time.Now().Format(time.RFC3339), name, cpr, key, logMsg)
	}

	// 3. Code Snippets
	fmt.Fprintf(out, "\n%s\n", codeComment)
	fmt.Fprintln(out, "const config = {")
	fmt.Fprintf(out, "  apiKey: '%s',\n", generateOpenAIKey())
	fmt.Fprintf(out, "  adminEmail: '%s',\n", emails[rand.Intn(len(emails))])
	fmt.Fprintln(out, "  environment: 'production',")
	fmt.Fprintln(out, "};")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "func ConnectToService() {")
	fmt.Fprintf(out, "  %s\n", secretComment)
	fmt.Fprintf(out, "  token := \"%s\"\n", generateOpenAIKey())
	fmt.Fprintln(out, "  fmt.Println(\"Connecting...\")")
	fmt.Fprintln(out, "}")

	fmt.Fprintf(out, "\n%s\n", endMsg)
}
