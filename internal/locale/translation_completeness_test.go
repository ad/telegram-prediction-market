package locale

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// extractMessageKeys extracts all message key constants from keys.go
func extractMessageKeys() ([]string, error) {
	keysFilePath := filepath.Join("keys.go")

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, keysFilePath, nil, parser.ParseComments)
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

// loadTranslationFile loads a translation JSON file and returns the keys
func loadTranslationFile(filename string) (map[string]interface{}, error) {
	filePath := filepath.Join("locales", filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var translations map[string]interface{}
	if err := json.Unmarshal(data, &translations); err != nil {
		return nil, err
	}

	return translations, nil
}

// isCommentKey checks if a key is a comment (starts with underscore)
func isCommentKey(key string) bool {
	return strings.HasPrefix(key, "_")
}

func TestProperty_TranslationKeyCompleteness(t *testing.T) {
	// Extract all message keys from keys.go
	messageKeys, err := extractMessageKeys()
	if err != nil {
		t.Fatalf("Failed to extract message keys: %v", err)
	}

	if len(messageKeys) == 0 {
		t.Fatal("No message keys found in keys.go")
	}

	// Load translation files
	ruTranslations, err := loadTranslationFile("ru.json")
	if err != nil {
		t.Fatalf("Failed to load ru.json: %v", err)
	}

	enTranslations, err := loadTranslationFile("en.json")
	if err != nil {
		t.Fatalf("Failed to load en.json: %v", err)
	}

	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("All message keys should have translations in both ru.json and en.json", prop.ForAll(
		func(keyIndex int) bool {
			// Use modulo to ensure valid index
			if len(messageKeys) == 0 {
				return true
			}
			keyIndex = keyIndex % len(messageKeys)
			if keyIndex < 0 {
				keyIndex = -keyIndex
			}

			key := messageKeys[keyIndex]

			// Check if key exists in Russian translations
			_, hasRu := ruTranslations[key]
			if !hasRu {
				t.Logf("Missing Russian translation for key: %s", key)
				return false
			}

			// Check if key exists in English translations
			_, hasEn := enTranslations[key]
			if !hasEn {
				t.Logf("Missing English translation for key: %s", key)
				return false
			}

			return true
		},
		gen.IntRange(0, len(messageKeys)*2), // Generate indices to sample keys
	))

	properties.TestingRun(t)
}

// TestAllTranslationKeysPresent is a comprehensive test that checks all keys
// This is not a property-based test, but a deterministic check of all keys
func TestAllTranslationKeysPresent(t *testing.T) {
	// Extract all message keys from keys.go
	messageKeys, err := extractMessageKeys()
	if err != nil {
		t.Fatalf("Failed to extract message keys: %v", err)
	}

	if len(messageKeys) == 0 {
		t.Fatal("No message keys found in keys.go")
	}

	t.Logf("Checking %d message keys for translation completeness", len(messageKeys))

	// Load translation files
	ruTranslations, err := loadTranslationFile("ru.json")
	if err != nil {
		t.Fatalf("Failed to load ru.json: %v", err)
	}

	enTranslations, err := loadTranslationFile("en.json")
	if err != nil {
		t.Fatalf("Failed to load en.json: %v", err)
	}

	var missingRu []string
	var missingEn []string

	for _, key := range messageKeys {
		// Check if key exists in Russian translations
		if _, hasRu := ruTranslations[key]; !hasRu {
			missingRu = append(missingRu, key)
		}

		// Check if key exists in English translations
		if _, hasEn := enTranslations[key]; !hasEn {
			missingEn = append(missingEn, key)
		}
	}

	if len(missingRu) > 0 {
		t.Errorf("Missing %d Russian translations:", len(missingRu))
		for _, key := range missingRu {
			t.Errorf("  - %s", key)
		}
	}

	if len(missingEn) > 0 {
		t.Errorf("Missing %d English translations:", len(missingEn))
		for _, key := range missingEn {
			t.Errorf("  - %s", key)
		}
	}

	// Also check for extra keys in translation files that don't exist in keys.go
	keySet := make(map[string]bool)
	for _, key := range messageKeys {
		keySet[key] = true
	}

	var extraRu []string
	var extraEn []string

	for key := range ruTranslations {
		// Skip comment keys
		if isCommentKey(key) {
			continue
		}
		if !keySet[key] {
			extraRu = append(extraRu, key)
		}
	}

	for key := range enTranslations {
		// Skip comment keys
		if isCommentKey(key) {
			continue
		}
		if !keySet[key] {
			extraEn = append(extraEn, key)
		}
	}

	if len(extraRu) > 0 {
		t.Logf("Warning: Found %d extra keys in ru.json not defined in keys.go:", len(extraRu))
		for _, key := range extraRu {
			t.Logf("  - %s", key)
		}
	}

	if len(extraEn) > 0 {
		t.Logf("Warning: Found %d extra keys in en.json not defined in keys.go:", len(extraEn))
		for _, key := range extraEn {
			t.Logf("  - %s", key)
		}
	}
}

// TestTranslationValueTypes checks that translation values are strings
func TestTranslationValueTypes(t *testing.T) {
	// Load translation files
	ruTranslations, err := loadTranslationFile("ru.json")
	if err != nil {
		t.Fatalf("Failed to load ru.json: %v", err)
	}

	enTranslations, err := loadTranslationFile("en.json")
	if err != nil {
		t.Fatalf("Failed to load en.json: %v", err)
	}

	// Check that all values are strings
	for key, value := range ruTranslations {
		if isCommentKey(key) {
			continue
		}
		if reflect.TypeOf(value).Kind() != reflect.String {
			t.Errorf("Russian translation for key %s is not a string: %v", key, value)
		}
	}

	for key, value := range enTranslations {
		if isCommentKey(key) {
			continue
		}
		if reflect.TypeOf(value).Kind() != reflect.String {
			t.Errorf("English translation for key %s is not a string: %v", key, value)
		}
	}
}
