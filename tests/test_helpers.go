package tests

import (
	"fmt"
	"math/rand"
)

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
