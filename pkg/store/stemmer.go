package store

import "strings"

// stem applies a simplified Porter Stemming algorithm to an English word.
// Returns the stemmed form.
func stem(word string) string {
	word = strings.ToLower(strings.TrimSpace(word))
	if len(word) < 3 {
		return word
	}

	// Step 1a: plurals
	if strings.HasSuffix(word, "sses") {
		word = word[:len(word)-2]
	} else if strings.HasSuffix(word, "ies") {
		word = word[:len(word)-2]
	} else if strings.HasSuffix(word, "ss") {
		// keep
	} else if strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "us") && !strings.HasSuffix(word, "is") {
		word = word[:len(word)-1]
	}

	// Step 1b: -ed, -ing
	if strings.HasSuffix(word, "eed") {
		// keep (only if >1 vowel, simplified)
	} else if strings.HasSuffix(word, "ed") && len(word) > 4 {
		word = word[:len(word)-2]
		if !hasVowel(word) {
			word += "e"
		}
	} else if strings.HasSuffix(word, "ing") && len(word) > 5 {
		word = word[:len(word)-3]
		if !hasVowel(word) {
			word += "e"
		}
	}

	// Step 1c: -y to -i
	if strings.HasSuffix(word, "y") && len(word) > 2 && hasVowel(word[:len(word)-1]) {
		word = word[:len(word)-1] + "i"
	}

	// Suffix removals (simplified)
	suffixes := []string{"ational", "tional", "enci", "anci", "izer", "ation",
		"ator", "alism", "iveness", "fulness", "ousness", "aliti", "iviti",
		"biliti", "logi", "entli", "ousli", "ization", "fulness", "iveness"}
	for _, suf := range suffixes {
		if strings.HasSuffix(word, suf) && len(word) > len(suf)+1 {
			word = word[:len(word)-len(suf)]
			break
		}
	}

	// -ate, -ize, -ble → remove if word > 3 chars
	for _, suf := range []string{"ate", "ize", "ble", "ive"} {
		if strings.HasSuffix(word, suf) && len(word) > len(suf)+1 {
			word = word[:len(word)-len(suf)]
			break
		}
	}

	return word
}

func hasVowel(s string) bool {
	for _, r := range s {
		switch r {
		case 'a', 'e', 'i', 'o', 'u', 'y':
			return true
		}
	}
	return false
}
