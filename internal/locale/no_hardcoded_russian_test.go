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

// containsCyrillic checks if a string contains Cyrillic characters
func containsCyrillic(s string) bool {
	for _, r := range s {
		if (r >= 0x0400 && r <= 0x04FF) || (r >= 0x0500 && r <= 0x052F) {
			return true
		}
	}
	return false
}

// isTestFile checks if a file is a test file
func isTestFile(filename string) bool {
	return strings.HasSuffix(filename, "_test.go")
}

// extractStringLiterals extracts all string literals from a Go source file
func extractStringLiterals(filePath string) ([]string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var literals []string
	ast.Inspect(node, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok {
			if lit.Kind == token.STRING {
				// Remove quotes from string literal
				value := lit.Value
				if len(value) >= 2 {
					value = value[1 : len(value)-1]
				}
				literals = append(literals, value)
			}
		}
		return true
	})

	return literals, nil
}

// findGoFiles recursively finds all Go source files in a directory
func findGoFiles(rootDir string, excludeTests bool) ([]string, error) {
	var files []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor and .git directories
		if info.IsDir() && (info.Name() == "vendor" || info.Name() == ".git") {
			return filepath.SkipDir
		}

		// Only process .go files
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			// Exclude test files if requested
			if excludeTests && isTestFile(path) {
				return nil
			}
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

func TestProperty_NoHardcodedRussianInProductionCode(t *testing.T) {
	// Find the project root (go up from internal/locale to project root)
	projectRoot := filepath.Join("..", "..")

	// Find all Go source files (excluding tests)
	files, err := findGoFiles(projectRoot, true)
	if err != nil {
		t.Fatalf("Failed to find Go files: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("No Go files found")
	}

	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Production code should not contain hardcoded Russian text", prop.ForAll(
		func(fileIndex int) bool {
			// Use modulo to ensure valid index
			if len(files) == 0 {
				return true
			}
			fileIndex = fileIndex % len(files)
			if fileIndex < 0 {
				fileIndex = -fileIndex
			}

			filePath := files[fileIndex]

			// Extract string literals from the file
			literals, err := extractStringLiterals(filePath)
			if err != nil {
				t.Logf("Failed to parse file %s: %v", filePath, err)
				return false
			}

			// Check each string literal for Cyrillic characters
			for _, literal := range literals {
				if containsCyrillic(literal) {
					t.Logf("Found hardcoded Russian text in %s: %q", filePath, literal)
					return false
				}
			}

			return true
		},
		gen.IntRange(0, len(files)*2), // Generate indices to sample files
	))

	properties.TestingRun(t)
}

// TestNoHardcodedRussianInAllFiles is a comprehensive test that checks all files
// This is not a property-based test, but a deterministic check of all files
func TestNoHardcodedRussianInAllFiles(t *testing.T) {
	// Find the project root
	projectRoot := filepath.Join("..", "..")

	// Find all Go source files (excluding tests)
	files, err := findGoFiles(projectRoot, true)
	if err != nil {
		t.Fatalf("Failed to find Go files: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("No Go files found")
	}

	t.Logf("Checking %d production Go files for hardcoded Russian text", len(files))

	var filesWithRussian []string
	var russianStrings []string

	for _, filePath := range files {
		// Extract string literals from the file
		literals, err := extractStringLiterals(filePath)
		if err != nil {
			t.Logf("Warning: Failed to parse file %s: %v", filePath, err)
			continue
		}

		// Check each string literal for Cyrillic characters
		for _, literal := range literals {
			if containsCyrillic(literal) {
				filesWithRussian = append(filesWithRussian, filePath)
				russianStrings = append(russianStrings, literal)
				t.Logf("Found hardcoded Russian text in %s: %q", filePath, literal)
			}
		}
	}

	if len(filesWithRussian) > 0 {
		t.Errorf("Found hardcoded Russian text in %d file(s)", len(filesWithRussian))
		for i, file := range filesWithRussian {
			t.Errorf("  %s: %q", file, russianStrings[i])
		}
	}
}
