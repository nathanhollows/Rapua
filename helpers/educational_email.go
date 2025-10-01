package helpers

import (
	"net/mail"
	"strings"
)

// Function to check if an email is educational.
func IsEducationalEmailHeuristic(email string) bool {
	// Validate the email format using net/mail
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil {
		// Invalid email format
		return false
	}

	// Extract the domain part from the parsed email
	atIndex := strings.LastIndex(parsedEmail.Address, "@")
	if atIndex == -1 || atIndex == len(parsedEmail.Address)-1 {
		// No domain part found
		return false
	}

	domain := strings.ToLower(parsedEmail.Address[atIndex+1:])

	// Check for invalid domain format
	if strings.HasPrefix(domain, ".") || strings.Contains(domain, "..") {
		// Invalid domain format
		return false
	}

	// Simple heuristic checks

	// 1. Check if domain ends with common educational TLDs
	eduTLDs := []string{
		".edu", ".ac", ".academy", ".school", ".college", ".university",
		".study", ".training", ".institute", ".museum", ".science",
	}
	for _, tld := range eduTLDs {
		if strings.HasSuffix(domain, tld) || strings.Contains(domain, tld+".") {
			return true
		}
	}

	// 2. Check for education-related keywords in the domain
	keywords := []string{
		"university", "college", "school", "academy", "institute", "education",
		"faculty", "campus", "students", "museum", "zoo", "botanical",
		"garden", "arboretum", "aquarium", "sciencecenter", "heritage",
		"conservatory", "gallery", "galleries",
	}
	for _, keyword := range keywords {
		if strings.Contains(domain, keyword) {
			return true
		}
	}

	return false
}
