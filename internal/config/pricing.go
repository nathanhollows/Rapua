package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	// DefaultCreditPriceCents is the default price per credit in cents.
	DefaultCreditPriceCents = 35
	// DefaultRegularUserFreeCredits is the default monthly free credits for regular users.
	DefaultRegularUserFreeCredits = 10
	// DefaultEducatorFreeCredits is the default monthly free credits for educators.
	DefaultEducatorFreeCredits = 25
)

// CreditPriceCents returns the price per credit in cents from environment variable.
// Falls back to DefaultCreditPriceCents if not set or invalid.
func CreditPriceCents() int {
	priceStr := os.Getenv("CREDIT_PRICE_CENTS")
	if priceStr == "" {
		return DefaultCreditPriceCents
	}

	price, err := strconv.Atoi(priceStr)
	if err != nil || price <= 0 {
		return DefaultCreditPriceCents
	}
	return price
}

// RegularUserFreeCredits returns the monthly free credits for regular users.
// Falls back to DefaultRegularUserFreeCredits if not set or invalid.
func RegularUserFreeCredits() int {
	creditsStr := os.Getenv("REGULAR_USER_FREE_CREDITS")
	if creditsStr == "" {
		return DefaultRegularUserFreeCredits
	}

	credits, err := strconv.Atoi(creditsStr)
	if err != nil || credits < 0 {
		return DefaultRegularUserFreeCredits
	}
	return credits
}

// EducatorFreeCredits returns the monthly free credits for educators.
// Falls back to DefaultEducatorFreeCredits if not set or invalid.
func EducatorFreeCredits() int {
	creditsStr := os.Getenv("EDUCATOR_FREE_CREDITS")
	if creditsStr == "" {
		return DefaultEducatorFreeCredits
	}

	credits, err := strconv.Atoi(creditsStr)
	if err != nil || credits < 0 {
		return DefaultEducatorFreeCredits
	}
	return credits
}

// CustomDomains parses CUSTOM_CREDIT_DOMAINS environment variable.
// Format: "@domain1.com:credits1,@domain2.edu:credits2"
// Returns map of domain -> credit amount.
func CustomDomains() map[string]int {
	domainsStr := os.Getenv("CUSTOM_CREDIT_DOMAINS")
	if domainsStr == "" {
		return make(map[string]int)
	}

	result := make(map[string]int)
	pairs := strings.Split(domainsStr, ",")

	const expectedParts = 2
	for _, pair := range pairs {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) != expectedParts {
			continue
		}

		domain := strings.TrimSpace(parts[0])
		creditsStr := strings.TrimSpace(parts[1])

		credits, err := strconv.Atoi(creditsStr)
		if err != nil || credits < 0 {
			continue
		}

		result[domain] = credits
	}

	return result
}

// GetFreeCreditsForEmail determines the monthly free credit allocation for a user based on their email.
// Priority: Custom domains → Educator email helper → Regular default.
// The isEducatorFunc parameter allows dependency injection of the educator email helper function.
func GetFreeCreditsForEmail(email string, isEducatorFunc func(string) bool) int {
	// Priority 1: Check custom domain overrides
	customDomains := CustomDomains()
	if len(customDomains) > 0 {
		// Extract domain from email
		atIndex := strings.LastIndex(email, "@")
		if atIndex != -1 && atIndex < len(email)-1 {
			domain := "@" + strings.ToLower(email[atIndex+1:])
			if credits, ok := customDomains[domain]; ok {
				return credits
			}
		}
	}

	// Priority 2: Check if email matches educator heuristic
	if isEducatorFunc != nil && isEducatorFunc(email) {
		return EducatorFreeCredits()
	}

	// Priority 3: Return regular default
	return RegularUserFreeCredits()
}
