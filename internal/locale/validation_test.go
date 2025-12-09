package locale

import (
	"testing"
)

// TestValidateTranslations tests the translation validation functionality
func TestValidateTranslations(t *testing.T) {
	result, err := ValidateTranslations()
	if err != nil {
		t.Fatalf("Validation failed with error: %v", err)
	}

	if result.HasErrors() {
		t.Logf("Validation found issues:\n%s", result.String())

		// Categorize errors
		missingCount := 0
		duplicateCount := 0
		unusedCount := 0
		duplicateKeyCount := 0
		duplicateJSONKeyCount := 0

		for _, err := range result.Errors {
			switch err.Type {
			case "missing_translation":
				missingCount++
			case "duplicate_translation":
				duplicateCount++
			case "unused_key":
				unusedCount++
			case "duplicate_key":
				duplicateKeyCount++
			case "duplicate_json_key":
				duplicateJSONKeyCount++
			}
		}

		if missingCount > 0 {
			t.Errorf("Found %d missing translations", missingCount)
		}
		if duplicateKeyCount > 0 {
			t.Errorf("Found %d duplicate key definitions in keys.go", duplicateKeyCount)
		}
		if duplicateJSONKeyCount > 0 {
			t.Errorf("Found %d duplicate keys in JSON files", duplicateJSONKeyCount)
		}
		if unusedCount > 0 {
			t.Errorf("Found %d unused keys in JSON files", unusedCount)
		}
		if duplicateCount > 0 {
			t.Logf("Found %d duplicate translations (warning only)", duplicateCount)
		}
	} else {
		t.Log("All translations are valid!")
	}
}

// TestNoMissingTranslations specifically checks for missing translations
func TestNoMissingTranslations(t *testing.T) {
	result, err := ValidateTranslations()
	if err != nil {
		t.Fatalf("Validation failed with error: %v", err)
	}

	var missingErrors []ValidationError
	for _, err := range result.Errors {
		if err.Type == "missing_translation" {
			missingErrors = append(missingErrors, err)
		}
	}

	if len(missingErrors) > 0 {
		t.Errorf("Found %d missing translations:", len(missingErrors))
		for _, err := range missingErrors {
			t.Errorf("  - %s", err.Message)
		}
	}
}

// TestDuplicateTranslations checks for duplicate translation values
func TestDuplicateTranslations(t *testing.T) {
	result, err := ValidateTranslations()
	if err != nil {
		t.Fatalf("Validation failed with error: %v", err)
	}

	var duplicateErrors []ValidationError
	for _, err := range result.Errors {
		if err.Type == "duplicate_translation" {
			duplicateErrors = append(duplicateErrors, err)
		}
	}

	if len(duplicateErrors) > 0 {
		t.Logf("Found %d duplicate translations (this may be intentional):", len(duplicateErrors))
		for _, err := range duplicateErrors {
			t.Logf("  - %s", err.Message)
		}
	}
}

// TestUnusedKeys checks for keys in JSON that are not in keys.go
func TestUnusedKeys(t *testing.T) {
	result, err := ValidateTranslations()
	if err != nil {
		t.Fatalf("Validation failed with error: %v", err)
	}

	var unusedErrors []ValidationError
	for _, err := range result.Errors {
		if err.Type == "unused_key" {
			unusedErrors = append(unusedErrors, err)
		}
	}

	if len(unusedErrors) > 0 {
		t.Logf("Found %d unused keys (these may need to be added to keys.go or removed from JSON):", len(unusedErrors))
		for _, err := range unusedErrors {
			t.Logf("  - %s", err.Message)
		}
	}
}

// TestNoUnusedKeysStrict is a strict version that fails if unused keys are found.
// This test ensures that all keys in translation JSON files (en.json, ru.json) are
// actually defined in keys.go. If a key exists in JSON but not in keys.go, it means:
// 1. The key was removed from keys.go but not from JSON files (cleanup needed)
// 2. The key was added to JSON files but not to keys.go (missing constant)
// This helps maintain consistency between code and translation files.
func TestNoUnusedKeysStrict(t *testing.T) {
	result, err := ValidateTranslations()
	if err != nil {
		t.Fatalf("Validation failed with error: %v", err)
	}

	var unusedErrors []ValidationError
	unusedByLanguage := make(map[string][]string)

	for _, err := range result.Errors {
		if err.Type == "unused_key" {
			unusedErrors = append(unusedErrors, err)
			if lang, ok := err.Details["language"].(string); ok {
				if key, ok := err.Details["key"].(string); ok {
					unusedByLanguage[lang] = append(unusedByLanguage[lang], key)
				}
			}
		}
	}

	if len(unusedErrors) > 0 {
		t.Errorf("Found %d unused keys in translation files:", len(unusedErrors))
		for lang, keys := range unusedByLanguage {
			t.Errorf("\n  %s.json has %d unused keys:", lang, len(keys))
			for _, key := range keys {
				t.Errorf("    - %s", key)
			}
		}
		t.Error("\nThese keys exist in JSON files but are not defined in keys.go.")
		t.Error("Either add them to keys.go or remove them from the JSON files.")
	}
}

// TestNoDuplicateKeys checks for duplicate key definitions in keys.go
func TestNoDuplicateKeys(t *testing.T) {
	result, err := ValidateTranslations()
	if err != nil {
		t.Fatalf("Validation failed with error: %v", err)
	}

	var duplicateKeyErrors []ValidationError
	for _, err := range result.Errors {
		if err.Type == "duplicate_key" {
			duplicateKeyErrors = append(duplicateKeyErrors, err)
		}
	}

	if len(duplicateKeyErrors) > 0 {
		t.Errorf("Found %d duplicate key definitions in keys.go:", len(duplicateKeyErrors))
		for _, err := range duplicateKeyErrors {
			t.Errorf("  - %s", err.Message)
		}
	}
}

// TestNoDuplicateJSONKeys checks for duplicate keys in JSON translation files
func TestNoDuplicateJSONKeys(t *testing.T) {
	result, err := ValidateTranslations()
	if err != nil {
		t.Fatalf("Validation failed with error: %v", err)
	}

	var duplicateJSONKeyErrors []ValidationError
	for _, err := range result.Errors {
		if err.Type == "duplicate_json_key" {
			duplicateJSONKeyErrors = append(duplicateJSONKeyErrors, err)
		}
	}

	if len(duplicateJSONKeyErrors) > 0 {
		t.Errorf("Found %d duplicate keys in JSON files:", len(duplicateJSONKeyErrors))
		for _, err := range duplicateJSONKeyErrors {
			t.Errorf("  - %s", err.Message)
		}
	}
}
