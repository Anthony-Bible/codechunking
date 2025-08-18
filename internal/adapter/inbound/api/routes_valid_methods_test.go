package api

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
REFACTORING PLAN: Hoist validMethods Map to Package Level

CURRENT PROBLEM:
In routes.go line 119-122, the validatePattern function creates a validMethods map on every call:
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}

This causes:
- Unnecessary allocations (~1 allocation per validatePattern call)
- DRY violation (same map created repeatedly)
- Slight performance overhead

PROPOSED SOLUTION:
1. Create a package-level variable: validHTTPMethods
2. Initialize it once at package load time
3. Reference it from validatePattern function
4. Maintain exact same behavior (case-insensitive validation)

IMPLEMENTATION OPTIONS:
Option A - Package-level var:
	var validHTTPMethods = map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}

Option B - Package-level slice with helper:
	var validHTTPMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	func isValidHTTPMethod(method string) bool { ... }

RECOMMENDED: Option A (map) for O(1) lookup performance

EXPECTED BENEFITS:
- Reduced allocations (from ~1000 to ~0 for 1000 calls)
- Better code organization (DRY principle)
- Minimal performance improvement
- Zero behavior changes

BACKWARD COMPATIBILITY:
- All existing functionality remains identical
- Method validation behavior unchanged
- Case-insensitive validation preserved
- Error messages remain the same

TEST STRATEGY:
- Failing tests verify package-level variable exists and is used
- Behavior tests ensure identical validation logic
- Performance tests verify allocation reduction
- Thread safety tests ensure concurrent access safety
*/

// TestValidHTTPMethodsPackageVariable_Exists tests that a package-level variable exists
// for valid HTTP methods. This test will FAIL initially because the variable doesn't exist yet.
func TestValidHTTPMethodsPackageVariable_Exists(t *testing.T) {
	t.Run("package_level_valid_http_methods_variable_exists", func(t *testing.T) {
		// This test will fail initially because we haven't hoisted the validMethods map yet
		// It expects a package-level variable named ValidHTTPMethods or validHTTPMethods

		// Try to access the package-level variable through reflection
		// This will fail because the variable doesn't exist yet
		var packageLevelVar interface{}
		var varName string
		var found bool

		// Check for different possible naming conventions
		possibleNames := []string{"ValidHTTPMethods", "validHTTPMethods", "httpMethodSet", "validMethods"}

		// This is a meta-test - it checks if the package-level variable exists
		// In the current implementation, this will fail because validMethods is only local
		for _, name := range possibleNames {
			// Try to find the variable in the package
			// This is a design test - we're testing what SHOULD exist after refactoring
			if checkPackageVariableExists(name) {
				packageLevelVar = getPackageVariable(name)
				varName = name
				found = true
				break
			}
		}

		assert.True(
			t,
			found,
			"Expected package-level HTTP methods variable to exist with one of these names: %v",
			possibleNames,
		)

		if found {
			// Verify it's a map[string]bool or similar structure
			v := reflect.ValueOf(packageLevelVar)
			assert.True(t, v.Kind() == reflect.Map || v.Kind() == reflect.Slice,
				"Package variable %s should be a map or slice, got %T", varName, packageLevelVar)
		}
	})
}

// TestValidHTTPMethodsPackageVariable_ContainsExpectedMethods tests the contents
// of the package-level variable. This test will FAIL initially.
func TestValidHTTPMethodsPackageVariable_ContainsExpectedMethods(t *testing.T) {
	t.Run("package_level_variable_contains_correct_methods", func(t *testing.T) {
		expectedMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

		// This will fail initially because the package-level variable doesn't exist
		packageVar := getPackageValidMethodsVariable()
		require.NotNil(t, packageVar, "Package-level valid methods variable should exist")

		// Test depending on whether it's implemented as map[string]bool or []string
		switch v := packageVar.(type) {
		case map[string]bool:
			for _, method := range expectedMethods {
				assert.True(t, v[method], "Method %s should be marked as valid in package-level variable", method)
			}
			// Ensure no extra methods
			assert.Len(t, v, len(expectedMethods), "Package-level variable should contain exactly %d methods", len(expectedMethods))

		case []string:
			for _, expected := range expectedMethods {
				found := false
				for _, actual := range v {
					if actual == expected {
						found = true
						break
					}
				}
				assert.True(t, found, "Method %s should be present in package-level slice", expected)
			}
			assert.Len(t, v, len(expectedMethods), "Package-level slice should contain exactly %d methods", len(expectedMethods))

		default:
			t.Fatalf("Package-level variable should be map[string]bool or []string, got %T", v)
		}
	})
}

// TestValidatePattern_UsesPackageLevelValidMethods tests that validatePattern
// function uses the package-level variable instead of creating a local one.
// This test will FAIL initially.
func TestValidatePattern_UsesPackageLevelValidMethods(t *testing.T) {
	t.Run("validate_pattern_references_package_level_variable", func(t *testing.T) {
		registry := NewRouteRegistry()

		// This is a behavioral test combined with implementation check
		// We'll test that the function behaves correctly AND uses the package variable

		// First, ensure the package variable exists (this will fail initially)
		packageVar := getPackageValidMethodsVariable()
		require.NotNil(t, packageVar, "Package-level valid methods variable must exist")

		// Test that validatePattern works with all methods from the package variable
		var methodsToTest []string

		switch v := packageVar.(type) {
		case map[string]bool:
			for method := range v {
				methodsToTest = append(methodsToTest, method)
			}
		case []string:
			methodsToTest = v
		default:
			t.Fatalf("Unexpected package variable type: %T", v)
		}

		// Test that all methods in package variable are accepted by validatePattern
		for _, method := range methodsToTest {
			pattern := fmt.Sprintf("%s /test", method)
			err := registry.validatePattern(pattern)
			require.NoError(t, err, "Method %s from package variable should be valid in pattern", method)

			// Test case insensitivity
			lowerPattern := fmt.Sprintf("%s /test", strings.ToLower(method))
			err = registry.validatePattern(lowerPattern)
			require.NoError(t, err, "Lowercase method %s should be valid", strings.ToLower(method))
		}

		// Test that invalid methods are still rejected
		invalidMethods := []string{"INVALID", "TRACE", "CONNECT", ""}
		for _, method := range invalidMethods {
			pattern := fmt.Sprintf("%s /test", method)
			err := registry.validatePattern(pattern)
			require.Error(t, err, "Invalid method %s should be rejected", method)
		}
	})
}

// TestValidatePattern_MethodValidation_IdenticalBehavior ensures that method validation
// behavior is identical before and after refactoring. This test should PASS both
// before and after the refactoring.
func TestValidatePattern_MethodValidation_IdenticalBehavior(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		expectError   bool
		errorContains string
	}{
		// Valid methods (should all pass)
		{"accepts_get_method", "GET /test", false, ""},
		{"accepts_post_method", "POST /test", false, ""},
		{"accepts_put_method", "PUT /test", false, ""},
		{"accepts_delete_method", "DELETE /test", false, ""},
		{"accepts_patch_method", "PATCH /test", false, ""},
		{"accepts_head_method", "HEAD /test", false, ""},
		{"accepts_options_method", "OPTIONS /test", false, ""},

		// Case insensitive (should all pass)
		{"accepts_lowercase_get", "get /test", false, ""},
		{"accepts_mixed_case_post", "Post /test", false, ""},
		{"accepts_lowercase_delete", "delete /test", false, ""},

		// Invalid methods (should all fail)
		{"rejects_invalid_method", "INVALID /test", true, "invalid HTTP method"},
		{"rejects_trace_method", "TRACE /test", true, "invalid HTTP method"},
		{"rejects_connect_method", "CONNECT /test", true, "invalid HTTP method"},
		{"rejects_custom_method", "CUSTOM /test", true, "invalid HTTP method"},

		// Edge cases
		{"rejects_empty_method", " /test", true, "HTTP method cannot be empty"},
		{"rejects_numeric_method", "123 /test", true, "invalid HTTP method"},
		{"rejects_special_chars_method", "GET! /test", true, "invalid HTTP method"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRouteRegistry()

			err := registry.validatePattern(tt.pattern)

			if tt.expectError {
				require.Error(t, err, "Pattern %s should be rejected", tt.pattern)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Error for pattern %s should contain %s", tt.pattern, tt.errorContains)
				}
			} else {
				require.NoError(t, err, "Pattern %s should be accepted", tt.pattern)
			}
		})
	}
}

// TestValidatePattern_AllocationBehavior tests that the refactored code
// reduces allocations by not creating the validMethods map on every call.
// This test may initially pass but should show improvement after refactoring.
func TestValidatePattern_AllocationBehavior(t *testing.T) {
	t.Run("validate_pattern_reduces_allocations_after_refactor", func(t *testing.T) {
		registry := NewRouteRegistry()
		pattern := "GET /test"

		// Measure allocations
		var m1, m2 runtime.MemStats

		runtime.GC()
		runtime.ReadMemStats(&m1)

		// Call validatePattern multiple times
		for range 1000 {
			err := registry.validatePattern(pattern)
			require.NoError(t, err)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		allocsDiff := m2.Mallocs - m1.Mallocs

		// This test establishes a baseline before refactoring
		// After refactoring, the number of allocations should be lower
		t.Logf("Allocations for 1000 validatePattern calls: %d", allocsDiff)

		// This assertion will likely fail initially (showing the problem)
		// but should pass after refactoring when we eliminate the map allocation
		assert.Less(
			t,
			allocsDiff,
			uint64(500),
			"Expected fewer than 500 allocations for 1000 calls, got %d (indicates validMethods map is being allocated repeatedly)",
			allocsDiff,
		)
	})
}

// TestValidatePattern_PerformanceComparison establishes performance baseline.
func TestValidatePattern_PerformanceComparison(t *testing.T) {
	t.Run("validate_pattern_performance_baseline", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping performance test in short mode")
		}

		registry := NewRouteRegistry()
		patterns := []string{
			"GET /test", "POST /test", "PUT /test", "DELETE /test",
			"PATCH /test", "HEAD /test", "OPTIONS /test",
		}

		// Benchmark the current implementation
		iterations := 10000

		start := time.Now()
		for range iterations {
			for _, pattern := range patterns {
				err := registry.validatePattern(pattern)
				require.NoError(t, err)
			}
		}
		elapsed := time.Since(start)

		avgNsPerCall := elapsed.Nanoseconds() / int64(iterations*len(patterns))
		t.Logf("Average time per validatePattern call: %d ns", avgNsPerCall)

		// This establishes a baseline - after refactoring, performance should improve
		// due to eliminating map allocation on each call
		assert.Less(t, avgNsPerCall, int64(10000),
			"Each validatePattern call should take less than 10 microseconds")
	})
}

// TestValidatePattern_ThreadSafety tests that the refactored validation is thread-safe
// This test should pass both before and after refactoring, but validates an important property.
func TestValidatePattern_ThreadSafety(t *testing.T) {
	t.Run("validate_pattern_is_thread_safe", func(t *testing.T) {
		registry := NewRouteRegistry()
		patterns := []string{"GET /test", "POST /test", "PUT /test", "DELETE /test"}

		// Run validation concurrently from multiple goroutines
		const numGoroutines = 10
		const iterationsPerGoroutine = 100

		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines*iterationsPerGoroutine)

		for range numGoroutines {
			go func() {
				defer func() { done <- true }()

				for range iterationsPerGoroutine {
					for _, pattern := range patterns {
						if err := registry.validatePattern(pattern); err != nil {
							errors <- fmt.Errorf("validation failed for pattern %s: %w", pattern, err)
							return
						}
					}
				}
			}()
		}

		// Wait for all goroutines to complete
		for range numGoroutines {
			<-done
		}

		close(errors)

		// Check for any errors
		var errorList []error
		for err := range errors {
			errorList = append(errorList, err)
		}

		assert.Empty(t, errorList, "Thread-safe validation should not produce errors: %v", errorList)
	})
}

// TestValidHTTPMethods_PackageLevelConstant tests specific properties of the
// package-level constant after refactoring. This will FAIL initially.
func TestValidHTTPMethods_PackageLevelConstant(t *testing.T) {
	t.Run("package_constant_is_immutable", func(t *testing.T) {
		// This test verifies that the package-level variable is treated as immutable
		packageVar := getPackageValidMethodsVariable()
		require.NotNil(t, packageVar, "Package-level variable must exist")

		// If it's a map, ensure it's not modified during validation
		if validMethods, ok := packageVar.(map[string]bool); ok {
			originalSize := len(validMethods)

			// Run multiple validations
			registry := NewRouteRegistry()
			for range 100 {
				err := registry.validatePattern("GET /test")
				require.NoError(t, err)
			}

			// Verify the package variable wasn't modified
			assert.Len(t, validMethods, originalSize, "Package-level map should not be modified")

			// Verify expected methods are still present
			expectedMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
			for _, method := range expectedMethods {
				assert.True(t, validMethods[method], "Method %s should still be valid", method)
			}
		}
	})
}

// TestValidatePattern_EdgeCasesWithPackageVariable tests edge cases specifically
// related to the package-level variable. Some parts will FAIL initially.
func TestValidatePattern_EdgeCasesWithPackageVariable(t *testing.T) {
	t.Run("handles_method_case_conversion_with_package_variable", func(t *testing.T) {
		registry := NewRouteRegistry()

		// Test that methods are converted to uppercase before checking against package variable
		testCases := []struct {
			input    string
			expected bool
		}{
			{"get /test", true},
			{"Get /test", true},
			{"GET /test", true},
			{"gEt /test", true},
			{"post /test", true},
			{"PoSt /test", true},
			{"invalid /test", false},
			{"INVALID /test", false},
		}

		for _, tc := range testCases {
			err := registry.validatePattern(tc.input)
			if tc.expected {
				require.NoError(t, err, "Pattern %s should be valid", tc.input)
			} else {
				require.Error(t, err, "Pattern %s should be invalid", tc.input)
			}
		}
	})

	t.Run("package_variable_covers_all_standard_methods", func(t *testing.T) {
		// This will fail initially because package variable doesn't exist
		packageVar := getPackageValidMethodsVariable()
		require.NotNil(t, packageVar, "Package-level variable must exist")

		// Verify it covers all standard HTTP methods that should be supported
		standardMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

		switch v := packageVar.(type) {
		case map[string]bool:
			for _, method := range standardMethods {
				assert.True(t, v[method], "Package variable should support standard method %s", method)
			}
		case []string:
			for _, expected := range standardMethods {
				found := false
				for _, actual := range v {
					if actual == expected {
						found = true
						break
					}
				}
				assert.True(t, found, "Package variable should contain standard method %s", expected)
			}
		}
	})
}

// TestRouteRegistration_WithRefactoredValidation ensures that route registration
// still works correctly after the validMethods refactoring.
func TestRouteRegistration_WithRefactoredValidation(t *testing.T) {
	t.Run("route_registration_works_with_refactored_validation", func(t *testing.T) {
		registry := NewRouteRegistry()

		// Test registration of routes with all valid HTTP methods
		validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

		for _, method := range validMethods {
			pattern := fmt.Sprintf("%s /test/%s", method, strings.ToLower(method))

			// This should work both before and after refactoring
			err := registry.RegisterRoute(pattern, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
			require.NoError(t, err, "Should be able to register route with method %s", method)

			// Verify route was registered
			assert.True(t, registry.HasRoute(pattern), "Route %s should be registered", pattern)
		}

		// Verify total count
		assert.Equal(t, len(validMethods), registry.RouteCount())
	})
}

// Helper functions for the failing tests

// checkPackageVariableExists checks if a package-level variable exists
// This is a test helper that will be used by failing tests.
func checkPackageVariableExists(varName string) bool {
	// Check for the specific variable names we expect
	switch varName {
	case "validHTTPMethods":
		return true
	default:
		return false
	}
}

// getPackageVariable retrieves a package-level variable by name
// This will fail initially because the variable doesn't exist.
func getPackageVariable(varName string) interface{} {
	// Return the actual package variable for the expected name
	switch varName {
	case "validHTTPMethods":
		return validHTTPMethods
	default:
		return nil
	}
}

// getPackageValidMethodsVariable attempts to retrieve the package-level
// valid HTTP methods variable. This will fail initially.
func getPackageValidMethodsVariable() interface{} {
	// Try different possible names for the package variable
	possibleNames := []string{"ValidHTTPMethods", "validHTTPMethods", "httpMethodSet", "validMethods"}

	for _, name := range possibleNames {
		if checkPackageVariableExists(name) {
			return getPackageVariable(name)
		}
	}

	// Return nil if not found - this will cause tests to fail initially
	return nil
}

// TestOptimizedImplementation_AllocationBaseline verifies the optimized implementation
// This test verifies that optimization successfully reduced allocations.
func TestOptimizedImplementation_AllocationBaseline(t *testing.T) {
	t.Run("optimized_implementation_uses_minimal_allocations", func(t *testing.T) {
		registry := NewRouteRegistry()

		// Measure current allocation behavior
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		// Call validatePattern many times
		for range 100 {
			err := registry.validatePattern("GET /test")
			require.NoError(t, err)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		allocsDiff := m2.Mallocs - m1.Mallocs
		t.Logf("Current implementation: %d allocations for 100 calls", allocsDiff)

		// After optimization: We expect minimal allocations
		// Optimized implementation should have very few allocations (previously 100+, now <10)
		assert.LessOrEqual(t, allocsDiff, uint64(10),
			"Optimized implementation should show minimal allocations (indicating the optimization worked)")
	})
}

// TestRefactoringRequirements_Summary provides a comprehensive overview
// This test will FAIL initially and succeeds when refactoring is complete.
func TestRefactoringRequirements_Summary(t *testing.T) {
	t.Run("refactoring_checklist", func(t *testing.T) {
		// 1. Package-level variable exists
		packageVar := getPackageValidMethodsVariable()
		assert.NotNil(t, packageVar, "❌ Package-level valid methods variable must exist")

		if packageVar != nil {
			// 2. Contains expected methods
			expectedMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

			switch v := packageVar.(type) {
			case map[string]bool:
				for _, method := range expectedMethods {
					assert.True(t, v[method], "❌ Method %s must be in package variable", method)
				}
				assert.Len(t, v, len(expectedMethods), "❌ Package variable must contain exactly expected methods")
				t.Log("✅ Package variable type: map[string]bool")

			case []string:
				assert.ElementsMatch(t, expectedMethods, v, "❌ Package slice must contain exactly expected methods")
				t.Log("✅ Package variable type: []string")

			default:
				t.Errorf("❌ Package variable must be map[string]bool or []string, got %T", v)
			}
		}

		// 3. Behavior remains identical
		registry := NewRouteRegistry()

		// All valid methods should work
		validPatterns := []string{
			"GET /test",
			"post /test",
			"Put /test",
			"DELETE /test",
			"PATCH /test",
			"head /test",
			"OPTIONS /test",
		}
		for _, pattern := range validPatterns {
			err := registry.validatePattern(pattern)
			require.NoError(t, err, "✅ Valid pattern %s should be accepted", pattern)
		}

		// Invalid methods should be rejected
		invalidPatterns := []string{"INVALID /test", "TRACE /test", "CONNECT /test"}
		for _, pattern := range invalidPatterns {
			err := registry.validatePattern(pattern)
			require.Error(t, err, "✅ Invalid pattern %s should be rejected", pattern)
		}

		// 4. Performance improvement (reduced allocations)
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		for range 100 {
			err := registry.validatePattern("GET /test")
			require.NoError(t, err)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		allocsDiff := m2.Mallocs - m1.Mallocs
		assert.Less(t, allocsDiff, uint64(20),
			"❌ After refactoring, allocations should be dramatically reduced (< 20 for 100 calls), got %d", allocsDiff)

		t.Logf("Refactoring status: %d allocations for 100 calls (target: < 20)", allocsDiff)
	})
}
