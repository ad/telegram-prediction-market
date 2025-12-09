package locale

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
)

// ValidationError represents a validation error found during initialization
type ValidationError struct {
	Type    string // "missing_translation", "duplicate_translation", "unused_key", "duplicate_key", "duplicate_json_key"
	Message string
	Details map[string]interface{}
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// ValidationResult contains all validation errors found
type ValidationResult struct {
	Errors []ValidationError
}

// HasErrors returns true if there are any validation errors
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// String returns a formatted string of all errors
func (r *ValidationResult) String() string {
	if !r.HasErrors() {
		return "No validation errors"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d validation errors:\n", len(r.Errors)))
	for i, err := range r.Errors {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, err.Error()))
	}
	return sb.String()
}

// ValidateTranslations performs comprehensive validation of translation files
func ValidateTranslations() (*ValidationResult, error) {
	result := &ValidationResult{}

	// Get the path to keys.go relative to this file
	_, filename, _, _ := runtime.Caller(0)
	keysPath := filepath.Join(filepath.Dir(filename), "keys.go")

	// Extract all message keys from keys.go
	messageKeys, err := extractMessageKeysFromFile(keysPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract message keys: %w", err)
	}

	// Load translation files
	translations := make(map[string]map[string]interface{})
	languages := []string{"en", "ru"}

	for _, lang := range languages {
		filename := fmt.Sprintf("locales/%s.json", lang)
		data, err := localizedata.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filename, err)
		}

		var trans map[string]interface{}
		if err := json.Unmarshal(data, &trans); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
		}

		translations[lang] = trans
	}

	// Check 1: Check for duplicate keys in keys.go
	result.checkDuplicateKeys(messageKeys)

	// Check 2: Check for duplicate keys in JSON files
	_, filename, _, _ = runtime.Caller(0)
	localesDir := filepath.Join(filepath.Dir(filename), "locales")
	for _, lang := range languages {
		jsonPath := filepath.Join(localesDir, fmt.Sprintf("%s.json", lang))
		result.checkDuplicateJSONKeys(jsonPath, lang)
	}

	// Check 3: All keys from keys.go should have translations in all languages
	result.checkMissingTranslations(messageKeys, translations, languages)

	// Check 4: Check for duplicate translation values
	result.checkDuplicateTranslations(translations, languages)

	// Check 5: Check for unused keys (keys in JSON but not in keys.go)
	result.checkUnusedKeys(messageKeys, translations, languages)

	return result, nil
}

// checkDuplicateKeys checks for duplicate key values in keys.go
func (r *ValidationResult) checkDuplicateKeys(messageKeys []string) {
	keyCount := make(map[string]int)
	keyPositions := make(map[string][]int)

	// Count occurrences of each key
	for i, key := range messageKeys {
		keyCount[key]++
		keyPositions[key] = append(keyPositions[key], i+1)
	}

	// Report duplicates
	for key, count := range keyCount {
		if count > 1 {
			r.Errors = append(r.Errors, ValidationError{
				Type:    "duplicate_key",
				Message: fmt.Sprintf("Duplicate key definition in keys.go: %s (appears %d times at positions %v)", key, count, keyPositions[key]),
				Details: map[string]interface{}{
					"key":       key,
					"count":     count,
					"positions": keyPositions[key],
				},
			})
		}
	}
}

// checkDuplicateJSONKeys checks for duplicate keys in a JSON file by parsing it manually
func (r *ValidationResult) checkDuplicateJSONKeys(jsonPath string, lang string) {
	// Read the JSON file as raw bytes
	data, err := localizedata.ReadFile(fmt.Sprintf("locales/%s.json", lang))
	if err != nil {
		r.Errors = append(r.Errors, ValidationError{
			Type:    "json_read_error",
			Message: fmt.Sprintf("Failed to read %s.json: %v", lang, err),
			Details: map[string]interface{}{
				"language": lang,
				"error":    err.Error(),
			},
		})
		return
	}

	// Parse the JSON and track keys
	var result map[string]interface{}
	seenKeys := make(map[string]bool)
	duplicateKeys := make(map[string]bool)

	// First pass: unmarshal normally
	if err := json.Unmarshal(data, &result); err != nil {
		r.Errors = append(r.Errors, ValidationError{
			Type:    "json_parse_error",
			Message: fmt.Sprintf("Failed to parse %s.json: %v", lang, err),
			Details: map[string]interface{}{
				"language": lang,
				"error":    err.Error(),
			},
		})
		return
	}

	// Second pass: manually scan for duplicate keys
	// This is a simple approach - scan the raw JSON for key patterns
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		// Look for JSON key pattern: "key":
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "\"") && strings.Contains(trimmed, "\":") {
			// Extract the key
			endQuote := strings.Index(trimmed[1:], "\"")
			if endQuote > 0 {
				key := trimmed[1 : endQuote+1]

				// Skip comment keys
				if strings.HasPrefix(key, "_") {
					continue
				}

				if seenKeys[key] {
					duplicateKeys[key] = true
				}
				seenKeys[key] = true
			}
		}
	}

	// Report duplicates
	for key := range duplicateKeys {
		r.Errors = append(r.Errors, ValidationError{
			Type:    "duplicate_json_key",
			Message: fmt.Sprintf("Duplicate key in %s.json: %s", lang, key),
			Details: map[string]interface{}{
				"language": lang,
				"key":      key,
			},
		})
	}
}

// checkMissingTranslations checks if all message keys have translations
func (r *ValidationResult) checkMissingTranslations(
	messageKeys []string,
	translations map[string]map[string]interface{},
	languages []string,
) {
	for _, key := range messageKeys {
		for _, lang := range languages {
			if _, exists := translations[lang][key]; !exists {
				r.Errors = append(r.Errors, ValidationError{
					Type:    "missing_translation",
					Message: fmt.Sprintf("Missing %s translation for key: %s", lang, key),
					Details: map[string]interface{}{
						"key":      key,
						"language": lang,
					},
				})
			}
		}
	}
}

// checkDuplicateTranslations checks for duplicate translation values
func (r *ValidationResult) checkDuplicateTranslations(
	translations map[string]map[string]interface{},
	languages []string,
) {
	for _, lang := range languages {
		trans := translations[lang]
		valueToKeys := make(map[string][]string)

		for key, value := range trans {
			// Skip comment keys
			if strings.HasPrefix(key, "_") {
				continue
			}

			strValue, ok := value.(string)
			if !ok {
				continue
			}

			// Normalize value for comparison (trim whitespace)
			normalized := strings.TrimSpace(strValue)
			if normalized == "" {
				continue
			}

			valueToKeys[normalized] = append(valueToKeys[normalized], key)
		}

		// Report duplicates
		for value, keys := range valueToKeys {
			if len(keys) > 1 {
				r.Errors = append(r.Errors, ValidationError{
					Type:    "duplicate_translation",
					Message: fmt.Sprintf("Duplicate %s translation value for keys: %v", lang, keys),
					Details: map[string]interface{}{
						"language": lang,
						"keys":     keys,
						"value":    value,
					},
				})
			}
		}
	}
}

// checkUnusedKeys checks for keys in translation files that are not defined in keys.go
func (r *ValidationResult) checkUnusedKeys(
	messageKeys []string,
	translations map[string]map[string]interface{},
	languages []string,
) {
	// Build a set of valid keys
	keySet := make(map[string]bool)
	for _, key := range messageKeys {
		keySet[key] = true
	}

	// Check each language for unused keys
	for _, lang := range languages {
		for key := range translations[lang] {
			// Skip comment keys
			if strings.HasPrefix(key, "_") {
				continue
			}

			if !keySet[key] {
				r.Errors = append(r.Errors, ValidationError{
					Type:    "unused_key",
					Message: fmt.Sprintf("Key %s exists in %s.json but not defined in keys.go", key, lang),
					Details: map[string]interface{}{
						"key":      key,
						"language": lang,
					},
				})
			}
		}
	}
}

// extractMessageKeysFromFile extracts all message key constants from keys.go
func extractMessageKeysFromFile(filename string) ([]string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var keys []string

	// Iterate through all declarations in the file
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}

		// Iterate through all specs in the const declaration
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			// Extract the constant value (the string literal)
			if len(valueSpec.Values) > 0 {
				if basicLit, ok := valueSpec.Values[0].(*ast.BasicLit); ok {
					if basicLit.Kind == token.STRING {
						// Remove quotes from string literal
						value := basicLit.Value
						if len(value) >= 2 {
							value = value[1 : len(value)-1]
						}
						keys = append(keys, value)
					}
				}
			}
		}
	}

	return keys, nil
}
