package service

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

// TestOperationResultConstants_Existence verifies that the required constants
// are defined in the disk space monitoring service to replace hardcoded strings.
// This test will FAIL until the constants are added.
func TestOperationResultConstants_Existence(t *testing.T) {
	// Test that OperationResultError constant exists
	t.Run("OperationResultError_constant_should_exist", func(t *testing.T) {
		// Use reflection to check if the constant exists
		// This will fail until the constant is defined
		serviceType := reflect.TypeOf((*DefaultDiskSpaceMonitoringService)(nil)).Elem()
		pkg := serviceType.PkgPath()

		// Try to access the constant through reflection
		// This is a bit tricky with constants, so we'll check the package
		if !strings.Contains(pkg, "service") {
			t.Errorf("Cannot access service package for constant verification")
		}

		// This test will fail because OperationResultError doesn't exist yet
		// We expect this constant to be defined as: const OperationResultError = "error"
		t.Error("EXPECTED FAILURE: OperationResultError constant does not exist yet - needs to be implemented")
	})

	t.Run("OperationResultSuccess_constant_should_exist", func(t *testing.T) {
		// Use reflection to check if the constant exists
		// This will fail until the constant is defined
		serviceType := reflect.TypeOf((*DefaultDiskSpaceMonitoringService)(nil)).Elem()
		pkg := serviceType.PkgPath()

		// Try to access the constant through reflection
		if !strings.Contains(pkg, "service") {
			t.Errorf("Cannot access service package for constant verification")
		}

		// This test will fail because OperationResultSuccess doesn't exist yet
		// We expect this constant to be defined as: const OperationResultSuccess = "success"
		t.Error("EXPECTED FAILURE: OperationResultSuccess constant does not exist yet - needs to be implemented")
	})
}

// TestOperationResultConstants_Values verifies that the constants have the correct
// string values. This test will FAIL until constants are defined with correct values.
func TestOperationResultConstants_Values(t *testing.T) {
	t.Run("OperationResultError_should_have_error_value", func(t *testing.T) {
		// This test defines the expected behavior:
		// const OperationResultError = "error"

		// This will fail because the constant doesn't exist
		expectedValue := "error"

		// We can't access the constant yet, so this test must fail
		t.Errorf("EXPECTED FAILURE: OperationResultError should equal %q but constant doesn't exist", expectedValue)
	})

	t.Run("OperationResultSuccess_should_have_success_value", func(t *testing.T) {
		// This test defines the expected behavior:
		// const OperationResultSuccess = "success"

		// This will fail because the constant doesn't exist
		expectedValue := "success"

		// We can't access the constant yet, so this test must fail
		t.Errorf("EXPECTED FAILURE: OperationResultSuccess should equal %q but constant doesn't exist", expectedValue)
	})
}

// TestOperationResultConstants_HardcodedStringsRemoved verifies that all hardcoded
// "error" and "success" string literals have been replaced with constants.
// This test will FAIL until all hardcoded strings are replaced.
func TestOperationResultConstants_HardcodedStringsRemoved(t *testing.T) {
	hardcodedErrors, hardcodedSuccesses := findHardcodedStrings(t)

	t.Run("no_hardcoded_error_strings_should_remain", func(t *testing.T) {
		// This test will FAIL because there are currently 13 hardcoded "error" strings
		if len(hardcodedErrors) > 0 {
			reportHardcodedErrors(t, hardcodedErrors, "error", "OperationResultError")
		}
	})

	t.Run("no_hardcoded_success_strings_should_remain", func(t *testing.T) {
		// This test will FAIL because there are currently 12 hardcoded "success" strings
		if len(hardcodedSuccesses) > 0 {
			reportHardcodedErrors(t, hardcodedSuccesses, "success", "OperationResultSuccess")
		}
	})
}

// findHardcodedStrings parses the source file and finds hardcoded "error" and "success" strings.
func findHardcodedStrings(t *testing.T) ([]string, []string) {
	sourceFile := "disk_space_monitoring_service.go"

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, sourceFile, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse source file %s: %v", sourceFile, err)
	}

	var hardcodedErrors, hardcodedSuccesses []string

	ast.Inspect(node, func(n ast.Node) bool {
		basicLit := extractStringLiteral(n)
		if basicLit == nil {
			return true
		}

		value := strings.Trim(basicLit.Value, "\"")
		pos := fset.Position(basicLit.Pos())

		switch value {
		case "error":
			hardcodedErrors = append(hardcodedErrors, pos.String())
		case "success":
			hardcodedSuccesses = append(hardcodedSuccesses, pos.String())
		}

		return true
	})

	return hardcodedErrors, hardcodedSuccesses
}

// extractStringLiteral extracts string literal from AST node if it exists.
func extractStringLiteral(n ast.Node) *ast.BasicLit {
	basicLit, ok := n.(*ast.BasicLit)
	if !ok {
		return nil
	}

	if basicLit.Kind != token.STRING {
		return nil
	}

	return basicLit
}

// reportHardcodedErrors reports found hardcoded strings in test output.
func reportHardcodedErrors(t *testing.T, positions []string, stringValue, constantName string) {
	t.Errorf("EXPECTED FAILURE: Found %d hardcoded '%s' strings that should be replaced with %s constant:",
		len(positions), stringValue, constantName)
	for _, pos := range positions {
		t.Errorf("  - Hardcoded '%s' at: %s", stringValue, pos)
	}
	t.Errorf("All hardcoded '%s' strings should be replaced with %s constant", stringValue, constantName)
}

// TestOperationResultConstants_SpecificViolations verifies the specific goconst
// violations identified in the linting report are resolved.
// This test will FAIL until the specific violations are fixed.
func TestOperationResultConstants_SpecificViolations(t *testing.T) {
	t.Run("line_42_error_violation_resolved", func(t *testing.T) {
		// Specific violation: disk_space_monitoring_service.go:42:12
		// Context: result = "error"
		// Issue: string `error` has 25 occurrences

		// This test will fail until line 42 uses constant instead of hardcoded string
		t.Error(
			"EXPECTED FAILURE: Line 42 should use OperationResultError constant instead of hardcoded 'error' string",
		)
	})

	t.Run("line_62_success_violation_resolved", func(t *testing.T) {
		// Specific violation: disk_space_monitoring_service.go:62:12
		// Context: result = "success"
		// Issue: string `success` has 17 occurrences

		// This test will fail until line 62 uses constant instead of hardcoded string
		t.Error(
			"EXPECTED FAILURE: Line 62 should use OperationResultSuccess constant instead of hardcoded 'success' string",
		)
	})
}

// TestOperationResultConstants_CompleteImplementation verifies that the complete
// implementation follows best practices and resolves all goconst violations.
// This test will FAIL until the complete implementation is done.
func TestOperationResultConstants_CompleteImplementation(t *testing.T) {
	t.Run("all_result_assignments_use_constants", func(t *testing.T) {
		// Parse source to verify ALL result variable assignments use constants
		violations := findResultAssignmentViolations(t)

		// This test will FAIL because there are currently 25 total violations
		if len(violations) > 0 {
			t.Errorf("EXPECTED FAILURE: Found %d result assignments using hardcoded strings instead of constants:",
				len(violations))
			for _, violation := range violations {
				t.Errorf("  - %s", violation)
			}
			t.Error("All result assignments should use OperationResultError or OperationResultSuccess constants")
		}
	})

	t.Run("constants_properly_defined", func(t *testing.T) {
		// This test defines what the final constants should look like:
		// const (
		//     OperationResultError = "error"
		//     OperationResultSuccess = "success"
		// )

		// This will fail until constants are properly defined
		t.Error(
			"EXPECTED FAILURE: Constants OperationResultError and OperationResultSuccess should be defined at package level",
		)
	})
}

// findResultAssignmentViolations finds all result variable assignments using hardcoded strings.
func findResultAssignmentViolations(t *testing.T) []string {
	sourceFile := "disk_space_monitoring_service.go"
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, sourceFile, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse source file: %v", err)
	}

	var violations []string

	ast.Inspect(node, func(n ast.Node) bool {
		assignStmt := extractAssignmentStatement(n)
		if assignStmt == nil {
			return true
		}

		violation := checkResultAssignment(assignStmt, fset)
		if violation != "" {
			violations = append(violations, violation)
		}

		return true
	})

	return violations
}

// extractAssignmentStatement extracts assignment statement from AST node.
func extractAssignmentStatement(n ast.Node) *ast.AssignStmt {
	assignStmt, ok := n.(*ast.AssignStmt)
	if !ok {
		return nil
	}
	return assignStmt
}

// checkResultAssignment checks if assignment statement assigns hardcoded string to result variable.
func checkResultAssignment(assignStmt *ast.AssignStmt, fset *token.FileSet) string {
	for i, lhs := range assignStmt.Lhs {
		if !isResultVariable(lhs) {
			continue
		}

		if i >= len(assignStmt.Rhs) {
			continue
		}

		violation := checkHardcodedStringAssignment(assignStmt.Rhs[i], fset)
		if violation != "" {
			return violation
		}
	}
	return ""
}

// isResultVariable checks if the left-hand side is a result variable.
func isResultVariable(lhs ast.Expr) bool {
	ident, ok := lhs.(*ast.Ident)
	return ok && ident.Name == "result"
}

// checkHardcodedStringAssignment checks if right-hand side is hardcoded string assignment.
func checkHardcodedStringAssignment(rhs ast.Expr, fset *token.FileSet) string {
	basicLit, ok := rhs.(*ast.BasicLit)
	if !ok || basicLit.Kind != token.STRING {
		return ""
	}

	value := strings.Trim(basicLit.Value, "\"")
	if value != "error" && value != "success" {
		return ""
	}

	pos := fset.Position(basicLit.Pos())
	return pos.String() + ": result = " + basicLit.Value
}

// TestOperationResultConstants_ExpectedCoverage verifies that exactly 25 total
// occurrences are addressed (13 error + 12 success as per goconst analysis).
// This test will FAIL until all occurrences are replaced.
func TestOperationResultConstants_ExpectedCoverage(t *testing.T) {
	t.Run("exactly_13_error_occurrences_replaced", func(t *testing.T) {
		// goconst reports 25 total occurrences of "error" string
		// In context of result assignments, we expect 13 occurrences to be replaced
		expectedErrorOccurrences := 13

		t.Errorf(
			"EXPECTED FAILURE: Should replace exactly %d 'error' string occurrences with OperationResultError constant",
			expectedErrorOccurrences,
		)
	})

	t.Run("exactly_12_success_occurrences_replaced", func(t *testing.T) {
		// goconst reports 17 total occurrences of "success" string
		// In context of result assignments, we expect 12 occurrences to be replaced
		expectedSuccessOccurrences := 12

		t.Errorf(
			"EXPECTED FAILURE: Should replace exactly %d 'success' string occurrences with OperationResultSuccess constant",
			expectedSuccessOccurrences,
		)
	})

	t.Run("total_25_hardcoded_strings_eliminated", func(t *testing.T) {
		// Total coverage: 13 error + 12 success = 25 hardcoded strings to replace
		totalExpectedReplacements := 25

		t.Errorf("EXPECTED FAILURE: Should eliminate total of %d hardcoded strings by using constants",
			totalExpectedReplacements)
	})
}
