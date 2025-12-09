package locale

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// localizerCall represents a call to a Localizer method
type localizerCall struct {
	file       string
	line       int
	method     string // "MustLocalize" or "MustLocalizeWithTemplate"
	messageKey string
}

// extractLocalizerCalls extracts all calls to Localizer methods from Go source files
func extractLocalizerCalls(rootDir string) ([]localizerCall, error) {
	var calls []localizerCall

	// Parse all Go files in the project (excluding test files for now, but we can include them)
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files and test files
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip vendor and .git directories
		if strings.Contains(path, "/vendor/") || strings.Contains(path, "/.git/") {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			// Skip files that can't be parsed
			return nil
		}

		// Walk the AST to find Localizer method calls
		ast.Inspect(node, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Check if this is a selector expression (e.g., localizer.MustLocalize)
			selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			// Check if the method is MustLocalize or MustLocalizeWithTemplate
			methodName := selExpr.Sel.Name
			if methodName != "MustLocalize" && methodName != "MustLocalizeWithTemplate" {
				return true
			}

			// Try to extract the message key (first argument)
			var messageKey string
			if len(callExpr.Args) > 0 {
				// Check if the first argument is a selector (e.g., locale.SomeKey)
				if selArg, ok := callExpr.Args[0].(*ast.SelectorExpr); ok {
					messageKey = selArg.Sel.Name
				}
			}

			// Record the call
			position := fset.Position(callExpr.Pos())
			calls = append(calls, localizerCall{
				file:       path,
				line:       position.Line,
				method:     methodName,
				messageKey: messageKey,
			})

			return true
		})

		return nil
	})

	return calls, err
}

func TestProperty_CorrectMethodUsageForNonParameterizedMessages(t *testing.T) {
	// Load translation files to determine which keys have parameters
	enTranslations, err := loadTranslationFile("en.json")
	if err != nil {
		t.Fatalf("Failed to load en.json: %v", err)
	}

	// Build a map of message keys to whether they have parameters
	keyHasParams := make(map[string]bool)
	for key, value := range enTranslations {
		if isCommentKey(key) {
			continue
		}
		if strVal, ok := value.(string); ok {
			keyHasParams[key] = hasTemplateParameters(strVal)
		}
	}

	// Extract all Localizer calls from the codebase
	calls, err := extractLocalizerCalls("../..")
	if err != nil {
		t.Fatalf("Failed to extract localizer calls: %v", err)
	}

	// Filter to only calls with identifiable message keys
	var validCalls []localizerCall
	for _, call := range calls {
		if call.messageKey != "" {
			validCalls = append(validCalls, call)
		}
	}

	if len(validCalls) == 0 {
		t.Skip("No Localizer calls with identifiable message keys found")
	}

	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Non-parameterized messages should use MustLocalize", prop.ForAll(
		func(callIndex int) bool {
			// Use modulo to ensure valid index
			if len(validCalls) == 0 {
				return true
			}
			callIndex = callIndex % len(validCalls)
			if callIndex < 0 {
				callIndex = -callIndex
			}

			call := validCalls[callIndex]

			// Check if this key has parameters
			hasParams, keyExists := keyHasParams[call.messageKey]
			if !keyExists {
				// Skip keys that don't exist in translations (might be from other code)
				return true
			}

			// If the message does NOT have parameters, it should use MustLocalize
			if !hasParams && call.method != "MustLocalize" {
				t.Logf("Message key %s has no parameters but uses %s at %s:%d",
					call.messageKey, call.method, call.file, call.line)
				return false
			}

			return true
		},
		gen.IntRange(0, len(validCalls)*2),
	))

	properties.TestingRun(t)
}

func TestProperty_CorrectMethodUsageForParameterizedMessages(t *testing.T) {
	// Load translation files to determine which keys have parameters
	enTranslations, err := loadTranslationFile("en.json")
	if err != nil {
		t.Fatalf("Failed to load en.json: %v", err)
	}

	// Build a map of message keys to whether they have parameters
	keyHasParams := make(map[string]bool)
	for key, value := range enTranslations {
		if isCommentKey(key) {
			continue
		}
		if strVal, ok := value.(string); ok {
			keyHasParams[key] = hasTemplateParameters(strVal)
		}
	}

	// Extract all Localizer calls from the codebase
	calls, err := extractLocalizerCalls("../..")
	if err != nil {
		t.Fatalf("Failed to extract localizer calls: %v", err)
	}

	// Filter to only calls with identifiable message keys
	var validCalls []localizerCall
	for _, call := range calls {
		if call.messageKey != "" {
			validCalls = append(validCalls, call)
		}
	}

	if len(validCalls) == 0 {
		t.Skip("No Localizer calls with identifiable message keys found")
	}

	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Parameterized messages should use MustLocalizeWithTemplate", prop.ForAll(
		func(callIndex int) bool {
			// Use modulo to ensure valid index
			if len(validCalls) == 0 {
				return true
			}
			callIndex = callIndex % len(validCalls)
			if callIndex < 0 {
				callIndex = -callIndex
			}

			call := validCalls[callIndex]

			// Check if this key has parameters
			hasParams, keyExists := keyHasParams[call.messageKey]
			if !keyExists {
				// Skip keys that don't exist in translations (might be from other code)
				return true
			}

			// If the message HAS parameters, it should use MustLocalizeWithTemplate
			if hasParams && call.method != "MustLocalizeWithTemplate" {
				t.Logf("Message key %s has parameters but uses %s at %s:%d",
					call.messageKey, call.method, call.file, call.line)
				return false
			}

			return true
		},
		gen.IntRange(0, len(validCalls)*2),
	))

	properties.TestingRun(t)
}

// TestAllLocalizerCallsUseCorrectMethod is a comprehensive test that checks all calls
// This is not a property-based test, but a deterministic check of all calls
func TestAllLocalizerCallsUseCorrectMethod(t *testing.T) {
	// Load translation files to determine which keys have parameters
	enTranslations, err := loadTranslationFile("en.json")
	if err != nil {
		t.Fatalf("Failed to load en.json: %v", err)
	}

	// Build a map of message keys to whether they have parameters
	keyHasParams := make(map[string]bool)
	for key, value := range enTranslations {
		if isCommentKey(key) {
			continue
		}
		if strVal, ok := value.(string); ok {
			keyHasParams[key] = hasTemplateParameters(strVal)
		}
	}

	// Extract all Localizer calls from the codebase
	calls, err := extractLocalizerCalls("../..")
	if err != nil {
		t.Fatalf("Failed to extract localizer calls: %v", err)
	}

	var errors []string
	checkedCount := 0

	for _, call := range calls {
		// Skip calls without identifiable message keys
		if call.messageKey == "" {
			continue
		}

		// Check if this key exists in translations
		hasParams, keyExists := keyHasParams[call.messageKey]
		if !keyExists {
			// This might be a key from test code or other sources
			continue
		}

		checkedCount++

		// Check if the correct method is used
		if hasParams && call.method != "MustLocalizeWithTemplate" {
			errors = append(errors,
				"Message key "+call.messageKey+" has parameters but uses "+call.method+
					" at "+call.file+":"+strconv.Itoa(call.line))
		} else if !hasParams && call.method != "MustLocalize" {
			errors = append(errors,
				"Message key "+call.messageKey+" has no parameters but uses "+call.method+
					" at "+call.file+":"+strconv.Itoa(call.line))
		}
	}

	t.Logf("Checked %d Localizer method calls", checkedCount)

	if len(errors) > 0 {
		t.Errorf("Found %d incorrect Localizer method usages:", len(errors))
		for _, err := range errors {
			t.Error(err)
		}
	}
}
