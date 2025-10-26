package helpers_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v5/helpers"
)

func TestIsEducationalEmail(t *testing.T) {
	testCases := []struct {
		email    string
		expected bool
	}{
		// Educational Institutions
		{"user@university.edu", true},
		{"student@college.ac.uk", true},
		{"teacher@school.edu.au", true},
		{"professor@institute.ac.in", true},
		{"towhomitmayconcern@otago.ac.nz", true},

		// Museums, Zoos, Botanical Gardens, Galleries
		{"staff@artgallery.org", true},
		{"info@nationalgallery.com", true},
		{"contact@citymuseum.net", true},
		{"employee@statezoo.org", true},
		{"gardener@botanicalgarden.edu", true},
		{"researcher@naturalhistorymuseum.org", true},
		{"curator@modernartgalleries.com", true},
		{"worker@aquarium.edu", true},
		{"staff@heritageconservatory.org", true},

		// Other Educational Entities
		{"member@onlineacademy.co", true},
		{"student@distancelearning.institute", true},
		{"faculty@medicalschool.edu", true},

		// Non-Educational Organizations
		{"user@business.com", false},
		{"contact@techcompany.net", false},
		{"support@ecommerce.shop", false},
		{"admin@corporate.org", false},

		// Personal Emails
		{"person@gmail.com", false},
		{"student@gmail.com", false},
		{"someone@yahoo.com", false},
		{"user@outlook.com", false},

		// Emails that slip through the heuristic
		{"info@museumcafe.com", true},
		{"sales@zoostore.net", true},
		{"contact@gardenservices.co", true},
		{"employee@artgalleryevents.com", true},
		{"user@example.education", true}, // Unhandled TLD
		{"user@university_ac_uk", true},  // Invalid format

		// Edge Cases
		{"", false},                             // Empty email
		{"invalidemail@", false},                // Missing domain
		{"@invalid.com", false},                 // Missing local part
		{"user@@example.com", false},            // Double '@'
		{"user@.edu", false},                    // Invalid domain
		{"user@university..edu", false},         // Consecutive dots
		{"user@university.ac.uk.", false},       // Trailing dot
		{"info@someuniversity.com", true},       // Contains 'university'
		{"contact@artgalleries.co.uk", true},    // Contains 'galleries'
		{"user@sciencemuseum.co.uk", true},      // Contains 'museum'
		{"staff@cityzoopark.org", true},         // Contains 'zoo'
		{"member@botanicalgardens.org", true},   // Contains 'botanical' and 'garden'
		{"employee@childrenmuseum.org", true},   // Contains 'museum'
		{"info@wildlifeconservatory.net", true}, // Contains 'conservatory'
		{"user@fashioninstitute.com", true},     // Contains 'institute'
		{"student@lawschool.edu", true},         // Contains 'school'
		{"user@mediacollege.org", true},         // Contains 'college'
		{"employee@citylibrary.org", false},     // Does not match any keyword
		{"user@museumcafe.org", true},           // Contains 'museum'
	}

	for _, tc := range testCases {
		result := helpers.IsEducationalEmailHeuristic(tc.email)
		if result != tc.expected {
			t.Errorf("IsEducationalEmail(%q) = %v; expected %v", tc.email, result, tc.expected)
		}
	}
}
