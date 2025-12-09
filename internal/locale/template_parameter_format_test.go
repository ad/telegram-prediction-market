package locale

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// extractTemplateParameters extracts all template parameters from a translation string
// Returns a slice of parameter numbers (e.g., [1, 2, 3] for "{{ .f1 }}", "{{ .f2 }}", "{{ .f3 }}")
func extractTemplateParameters(translation string) ([]int, error) {
	// Match template parameters like {{ .f1 }}, {{ .f2 }}, etc.
	re := regexp.MustCompile(`\{\{\s*\.f(\d+)\s*\}\}`)
	matches := re.FindAllStringSubmatch(translation, -1)

	var params []int
	seen := make(map[int]bool)

	for _, match := range matches {
		if len(match) >= 2 {
			num, err := strconv.Atoi(match[1])
			if err != nil {
				return nil, err
			}
			// Only add unique parameter numbers
			if !seen[num] {
				params = append(params, num)
				seen[num] = true
			}
		}
	}

	// Sort parameters to check sequential order
	sort.Ints(params)

	return params, nil
}

// isSequential checks if a slice of integers is sequential starting from 1
func isSequential(params []int) bool {
	if len(params) == 0 {
		return true // No parameters is valid
	}

	// Must start with 1
	if params[0] != 1 {
		return false
	}

	// Check that each subsequent number is exactly 1 more than the previous
	for i := 1; i < len(params); i++ {
		if params[i] != params[i-1]+1 {
			return false
		}
	}

	return true
}

// hasTemplateParameters checks if a translation string contains any template parameters
func hasTemplateParameters(translation string) bool {
	return strings.Contains(translation, "{{ .f")
}

func TestProperty_TemplateParameterFormat(t *testing.T) {
	// Load translation files
	ruTranslations, err := loadTranslationFile("ru.json")
	if err != nil {
		t.Fatalf("Failed to load ru.json: %v", err)
	}

	enTranslations, err := loadTranslationFile("en.json")
	if err != nil {
		t.Fatalf("Failed to load en.json: %v", err)
	}

	// Collect all translation keys that have parameters
	var keysWithParams []string
	for key, value := range enTranslations {
		if isCommentKey(key) {
			continue
		}
		if strVal, ok := value.(string); ok {
			if hasTemplateParameters(strVal) {
				keysWithParams = append(keysWithParams, key)
			}
		}
	}

	if len(keysWithParams) == 0 {
		t.Skip("No translation keys with template parameters found")
	}

	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Template parameters should follow sequential format {{ .f1 }}, {{ .f2 }}, etc.", prop.ForAll(
		func(keyIndex int) bool {
			// Use modulo to ensure valid index
			if len(keysWithParams) == 0 {
				return true
			}
			keyIndex = keyIndex % len(keysWithParams)
			if keyIndex < 0 {
				keyIndex = -keyIndex
			}

			key := keysWithParams[keyIndex]

			// Check English translation
			if enVal, ok := enTranslations[key]; ok {
				if enStr, ok := enVal.(string); ok {
					params, err := extractTemplateParameters(enStr)
					if err != nil {
						t.Logf("Error extracting parameters from English translation for key %s: %v", key, err)
						return false
					}

					if !isSequential(params) {
						t.Logf("English translation for key %s has non-sequential parameters: %v\nTranslation: %s", key, params, enStr)
						return false
					}
				}
			}

			// Check Russian translation
			if ruVal, ok := ruTranslations[key]; ok {
				if ruStr, ok := ruVal.(string); ok {
					params, err := extractTemplateParameters(ruStr)
					if err != nil {
						t.Logf("Error extracting parameters from Russian translation for key %s: %v", key, err)
						return false
					}

					if !isSequential(params) {
						t.Logf("Russian translation for key %s has non-sequential parameters: %v\nTranslation: %s", key, params, ruStr)
						return false
					}
				}
			}

			return true
		},
		gen.IntRange(0, len(keysWithParams)*2), // Generate indices to sample keys
	))

	properties.TestingRun(t)
}

// TestAllTemplateParametersSequential is a comprehensive test that checks all keys
// This is not a property-based test, but a deterministic check of all keys
func TestAllTemplateParametersSequential(t *testing.T) {
	// Load translation files
	ruTranslations, err := loadTranslationFile("ru.json")
	if err != nil {
		t.Fatalf("Failed to load ru.json: %v", err)
	}

	enTranslations, err := loadTranslationFile("en.json")
	if err != nil {
		t.Fatalf("Failed to load en.json: %v", err)
	}

	var errors []string

	// Check all English translations
	for key, value := range enTranslations {
		if isCommentKey(key) {
			continue
		}

		if strVal, ok := value.(string); ok {
			if hasTemplateParameters(strVal) {
				params, err := extractTemplateParameters(strVal)
				if err != nil {
					errors = append(errors, "Error extracting parameters from English translation for key "+key+": "+err.Error())
					continue
				}

				if !isSequential(params) {
					errors = append(errors, "English translation for key "+key+" has non-sequential parameters: "+formatParams(params)+"\nTranslation: "+strVal)
				}
			}
		}
	}

	// Check all Russian translations
	for key, value := range ruTranslations {
		if isCommentKey(key) {
			continue
		}

		if strVal, ok := value.(string); ok {
			if hasTemplateParameters(strVal) {
				params, err := extractTemplateParameters(strVal)
				if err != nil {
					errors = append(errors, "Error extracting parameters from Russian translation for key "+key+": "+err.Error())
					continue
				}

				if !isSequential(params) {
					errors = append(errors, "Russian translation for key "+key+" has non-sequential parameters: "+formatParams(params)+"\nTranslation: "+strVal)
				}
			}
		}
	}

	if len(errors) > 0 {
		t.Errorf("Found %d template parameter format errors:", len(errors))
		for _, err := range errors {
			t.Error(err)
		}
	}
}

// TestParameterConsistencyBetweenLanguages checks that both languages use the same parameters
func TestParameterConsistencyBetweenLanguages(t *testing.T) {
	// Load translation files
	ruTranslations, err := loadTranslationFile("ru.json")
	if err != nil {
		t.Fatalf("Failed to load ru.json: %v", err)
	}

	enTranslations, err := loadTranslationFile("en.json")
	if err != nil {
		t.Fatalf("Failed to load en.json: %v", err)
	}

	var errors []string

	// Check that both languages have the same parameters for each key
	for key, enValue := range enTranslations {
		if isCommentKey(key) {
			continue
		}

		enStr, enOk := enValue.(string)
		if !enOk {
			continue
		}

		ruValue, ruExists := ruTranslations[key]
		if !ruExists {
			continue // Skip if Russian translation doesn't exist (handled by completeness test)
		}

		ruStr, ruOk := ruValue.(string)
		if !ruOk {
			continue
		}

		// Extract parameters from both translations
		enParams, err := extractTemplateParameters(enStr)
		if err != nil {
			errors = append(errors, "Error extracting parameters from English translation for key "+key+": "+err.Error())
			continue
		}

		ruParams, err := extractTemplateParameters(ruStr)
		if err != nil {
			errors = append(errors, "Error extracting parameters from Russian translation for key "+key+": "+err.Error())
			continue
		}

		// Check if parameter counts match
		if len(enParams) != len(ruParams) {
			errors = append(errors, "Parameter count mismatch for key "+key+":\n  English: "+formatParams(enParams)+"\n  Russian: "+formatParams(ruParams))
			continue
		}

		// Check if parameters are the same
		for i := range enParams {
			if enParams[i] != ruParams[i] {
				errors = append(errors, "Parameter mismatch for key "+key+":\n  English: "+formatParams(enParams)+"\n  Russian: "+formatParams(ruParams))
				break
			}
		}
	}

	if len(errors) > 0 {
		t.Errorf("Found %d parameter consistency errors:", len(errors))
		for _, err := range errors {
			t.Error(err)
		}
	}
}

// formatParams formats a slice of parameter numbers for display
func formatParams(params []int) string {
	if len(params) == 0 {
		return "none"
	}

	var parts []string
	for _, p := range params {
		parts = append(parts, "{{ .f"+strconv.Itoa(p)+" }}")
	}
	return strings.Join(parts, ", ")
}
