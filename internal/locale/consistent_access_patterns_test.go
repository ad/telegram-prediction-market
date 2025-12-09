package locale

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// localizerCallPattern represents the pattern of a Localizer method call
type localizerCallPattern struct {
	file           string
	line           int
	method         string // "MustLocalize" or "MustLocalizeWithTemplate"
	usesConstant   bool   // true if first arg is a constant from locale package
	firstArgString string // the actual first argument as string
}

// extractLocalizerCallPatterns extracts all Localizer method calls and analyzes their patterns
func extractLocalizerCallPatterns(rootDir string) ([]localizerCallPattern, error) {
	var patterns []localizerCallPattern

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

			// Analyze the first argument
			var usesConstant bool
			var firstArgString string

			if len(callExpr.Args) > 0 {
				firstArg := callExpr.Args[0]

				// Check if the first argument is a selector (e.g., locale.SomeKey or just SomeKey)
				if selArg, ok := firstArg.(*ast.SelectorExpr); ok {
					// This is a qualified identifier like locale.SomeKey
					if ident, ok := selArg.X.(*ast.Ident); ok {
						// Check if it's from the locale package
						if ident.Name == "locale" {
							usesConstant = true
							firstArgString = selArg.Sel.Name
						}
					}
				} else if ident, ok := firstArg.(*ast.Ident); ok {
					// This is an unqualified identifier (constant from same package or imported)
					// We'll consider this as using a constant
					usesConstant = true
					firstArgString = ident.Name
				} else if basicLit, ok := firstArg.(*ast.BasicLit); ok {
					// This is a string literal - NOT using a constant
					usesConstant = false
					firstArgString = basicLit.Value
				}
			}

			// Record the pattern
			position := fset.Position(callExpr.Pos())
			patterns = append(patterns, localizerCallPattern{
				file:           path,
				line:           position.Line,
				method:         methodName,
				usesConstant:   usesConstant,
				firstArgString: firstArgString,
			})

			return true
		})

		return nil
	})

	return patterns, err
}

func TestProperty_ConsistentLocalizerAccessPatterns(t *testing.T) {
	// Extract all Localizer call patterns from the codebase
	patterns, err := extractLocalizerCallPatterns("../..")
	if err != nil {
		t.Fatalf("Failed to extract localizer call patterns: %v", err)
	}

	if len(patterns) == 0 {
		t.Skip("No Localizer calls found")
	}

	// Filter to only patterns with identifiable first arguments
	var validPatterns []localizerCallPattern
	for _, pattern := range patterns {
		if pattern.firstArgString != "" {
			validPatterns = append(validPatterns, pattern)
		}
	}

	if len(validPatterns) == 0 {
		t.Skip("No Localizer calls with identifiable arguments found")
	}

	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("All Localizer calls should use constants from keys.go", prop.ForAll(
		func(patternIndex int) bool {
			// Use modulo to ensure valid index
			if len(validPatterns) == 0 {
				return true
			}
			patternIndex = patternIndex % len(validPatterns)
			if patternIndex < 0 {
				patternIndex = -patternIndex
			}

			pattern := validPatterns[patternIndex]

			// Check if the call uses a constant (not a string literal)
			if !pattern.usesConstant {
				t.Logf("Localizer call at %s:%d uses string literal instead of constant: %s",
					pattern.file, pattern.line, pattern.firstArgString)
				return false
			}

			return true
		},
		gen.IntRange(0, len(validPatterns)*2),
	))

	properties.TestingRun(t)
}

// TestAllLocalizerCallsUseConstants is a comprehensive test that checks all calls
// This is not a property-based test, but a deterministic check of all calls
func TestAllLocalizerCallsUseConstants(t *testing.T) {
	// Extract all Localizer call patterns from the codebase
	patterns, err := extractLocalizerCallPatterns("../..")
	if err != nil {
		t.Fatalf("Failed to extract localizer call patterns: %v", err)
	}

	if len(patterns) == 0 {
		t.Skip("No Localizer calls found")
	}

	t.Logf("Checking %d Localizer calls for consistent access patterns", len(patterns))

	var violations []string
	checkedCount := 0

	for _, pattern := range patterns {
		// Skip patterns without identifiable first arguments
		if pattern.firstArgString == "" {
			continue
		}

		checkedCount++

		// Check if the call uses a constant (not a string literal)
		if !pattern.usesConstant {
			violations = append(violations,
				"Localizer call at "+pattern.file+":"+
					filepath.Base(pattern.file)+":"+
					"line "+formatInt(pattern.line)+
					" uses string literal instead of constant: "+pattern.firstArgString)
		}
	}

	t.Logf("Checked %d Localizer calls with identifiable arguments", checkedCount)

	if len(violations) > 0 {
		t.Errorf("Found %d Localizer calls that don't use constants:", len(violations))
		for _, violation := range violations {
			t.Error(violation)
		}
		t.Error("\nAll Localizer calls should use constants from internal/locale/keys.go")
		t.Error("Example: localizer.MustLocalize(locale.HelpTitle) instead of localizer.MustLocalize(\"HelpTitle\")")
	}
}

// formatInt converts an int to string (helper function)
func formatInt(n int) string {
	if n < 0 {
		return "-" + formatInt(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return formatInt(n/10) + string(rune('0'+n%10))
}
