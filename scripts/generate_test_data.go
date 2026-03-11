package main

import (
	"flag"
	"fmt"
	"math/rand"
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
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

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

	fmt.Println(title)
	fmt.Println()

	// 1. Support Emails
	for i := 0; i < 3; i++ {
		name := names[rand.Intn(len(names))]
		email := emails[rand.Intn(len(emails))]
		cpr := generateCPR()
		fmt.Printf("%s%d\n", subjectPrefix, 1000+i)
		fmt.Printf("%s%s <%s>\n", fromPrefix, name, email)
		fmt.Printf(emailBody1, name)
		fmt.Printf(emailBody2, email, cpr)
		fmt.Println(emailBody3 + name)
		fmt.Println("---")
	}

	// 2. JSON Logs
	fmt.Println("\n// System Logs (JSON format)")
	for i := 0; i < 5; i++ {
		name := names[rand.Intn(len(names))]
		cpr := generateCPR()
		key := generateOpenAIKey()
		fmt.Printf("{\"timestamp\": \"%s\", \"level\": \"INFO\", \"user\": \"%s\", \"cpr\": \"%s\", \"api_key\": \"%s\", \"msg\": \"%s\"}\n", 
			time.Now().Format(time.RFC3339), name, cpr, key, logMsg)
	}

	// 3. Code Snippets
	fmt.Printf("\n%s\n", codeComment)
	fmt.Println("const config = {")
	fmt.Printf("  apiKey: '%s',\n", generateOpenAIKey())
	fmt.Printf("  adminEmail: '%s',\n", emails[rand.Intn(len(emails))])
	fmt.Println("  environment: 'production',")
	fmt.Println("};")
	fmt.Println()

	fmt.Println("func ConnectToService() {")
	fmt.Printf("  %s\n", secretComment)
	fmt.Printf("  token := \"%s\"\n", generateOpenAIKey())
	fmt.Println("  fmt.Println(\"Connecting...\")")
	fmt.Println("}")

	fmt.Printf("\n%s\n", endMsg)
}
