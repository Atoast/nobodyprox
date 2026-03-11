package main

import (
	"fmt"
	"math/rand"
	"time"
)

var names = []string{"Alice Smith", "Bob Jones", "Charlie Brown", "David Wilson", "Eva Hansen", "Frederik Nielsen", "Gitte Jensen", "Hans Pedersen"}
var emails = []string{"alice@example.com", "bob.j@gmail.com", "charlie_b@outlook.dk", "david.wilson@company.com", "eva_h@hansen.dk"}
var domains = []string{"example.com", "gmail.com", "company.org", "service.io"}

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
	rand.Seed(time.Now().UnixNano())

	fmt.Println("--- Synthetic Test Data for NobodyProx ---")
	fmt.Println()

	// 1. Support Emails
	for i := 0; i < 3; i++ {
		name := names[rand.Intn(len(names))]
		email := emails[rand.Intn(len(emails))]
		cpr := generateCPR()
		fmt.Printf("Subject: Support Request #%d\n", 1000+i)
		fmt.Printf("From: %s <%s>\n", name, email)
		fmt.Printf("Hi support,\n\nI am %s, and I am having trouble with my account.\n", name)
		fmt.Printf("My registered email is %s and my Danish CPR for verification is %s.\n", email, cpr)
		fmt.Println("Please help me reset my password.\n\nBest regards,\n" + name)
		fmt.Println("---")
	}

	// 2. JSON Logs
	fmt.Println("\n// System Logs (JSON format)")
	for i := 0; i < 5; i++ {
		name := names[rand.Intn(len(names))]
		cpr := generateCPR()
		key := generateOpenAIKey()
		fmt.Printf("{\"timestamp\": \"%s\", \"level\": \"INFO\", \"user\": \"%s\", \"cpr\": \"%s\", \"api_key\": \"%s\", \"msg\": \"User accessed secure resource\"}\n", 
			time.Now().Format(time.RFC3339), name, cpr, key)
	}

	// 3. Code Snippets
	fmt.Println("\n// Application Code Snippets")
	fmt.Println("const config = {")
	fmt.Printf("  apiKey: '%s',\n", generateOpenAIKey())
	fmt.Printf("  adminEmail: '%s',\n", emails[rand.Intn(len(emails))])
	fmt.Println("  environment: 'production',")
	fmt.Println("};")
	fmt.Println()

	fmt.Println("func ConnectToService() {")
	fmt.Printf("  // Hardcoded secret for testing\n")
	fmt.Printf("  token := \"%s\"\n", generateOpenAIKey())
	fmt.Println("  fmt.Println(\"Connecting...\")")
	fmt.Println("}")

	fmt.Println("\n--- End of Test Data ---")
}
