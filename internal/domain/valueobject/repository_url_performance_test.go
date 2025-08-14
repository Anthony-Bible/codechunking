package valueobject

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRepositoryURL_NormalizationPerformance tests that normalization only happens once.
func TestRepositoryURL_NormalizationPerformance(t *testing.T) {
	testCases := []struct {
		name     string
		rawInput string
	}{
		{
			name:     "trailing slash URL",
			rawInput: "https://github.com/golang/go/",
		},
		{
			name:     "git suffix URL",
			rawInput: "https://github.com/golang/go.git",
		},
		{
			name:     "mixed case URL",
			rawInput: "https://GitHub.com/GoLang/Go",
		},
		{
			name:     "URL with query params",
			rawInput: "https://github.com/golang/go?tab=readme",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create RepositoryURL - normalization should happen here
			start := time.Now()
			repoURL, err := NewRepositoryURL(tc.rawInput)
			creationTime := time.Since(start)
			require.NoError(t, err, "Failed to create RepositoryURL")

			// Multiple calls to Raw(), Normalized(), and String() should be fast
			// because normalization was already done during creation

			// THIS WILL FAIL because Raw() method doesn't exist yet
			start = time.Now()
			for range 1000 {
				_ = repoURL.Raw()
			}
			rawCallsTime := time.Since(start)

			// THIS WILL FAIL because Normalized() method doesn't exist yet
			start = time.Now()
			for range 1000 {
				_ = repoURL.Normalized()
			}
			normalizedCallsTime := time.Since(start)

			// String() calls should also be fast
			start = time.Now()
			for range 1000 {
				_ = repoURL.String()
			}
			stringCallsTime := time.Since(start)

			t.Logf("Creation time: %v", creationTime)
			t.Logf("1000 Raw() calls time: %v", rawCallsTime)
			t.Logf("1000 Normalized() calls time: %v", normalizedCallsTime)
			t.Logf("1000 String() calls time: %v", stringCallsTime)

			// Method calls should be significantly faster than creation
			// because normalization happens only during creation
			assert.Less(t, rawCallsTime, creationTime,
				"Raw() calls should be faster than creation (no re-normalization)")
			assert.Less(t, normalizedCallsTime, creationTime,
				"Normalized() calls should be faster than creation (no re-normalization)")
			assert.Less(t, stringCallsTime, creationTime,
				"String() calls should be faster than creation (no re-normalization)")

			// All method calls should be very fast (sub-millisecond for 1000 calls)
			maxMethodTime := 10 * time.Millisecond
			assert.Less(t, rawCallsTime, maxMethodTime,
				"1000 Raw() calls should be very fast")
			assert.Less(t, normalizedCallsTime, maxMethodTime,
				"1000 Normalized() calls should be very fast")
			assert.Less(t, stringCallsTime, maxMethodTime,
				"1000 String() calls should be very fast")
		})
	}
}

// TestRepositoryURL_NoNormalizationOnMethodCalls tests that methods don't perform normalization.
func TestRepositoryURL_NoNormalizationOnMethodCalls(t *testing.T) {
	// Create a RepositoryURL that needs normalization
	rawInput := "https://GitHub.com/GoLang/Go.git"
	repoURL, err := NewRepositoryURL(rawInput)
	require.NoError(t, err, "Failed to create RepositoryURL")

	// Benchmark single method calls to ensure they're not doing normalization
	t.Run("Raw_method_is_constant_time", func(t *testing.T) {
		// THIS WILL FAIL because Raw() method doesn't exist yet
		times := make([]time.Duration, 100)
		for i := range 100 {
			start := time.Now()
			_ = repoURL.Raw()
			times[i] = time.Since(start)
		}

		// All calls should be very fast and consistent (no normalization)
		for _, duration := range times {
			assert.Less(t, duration, 100*time.Microsecond,
				"Raw() call should be very fast (no normalization)")
		}

		// Verify consistency - standard deviation should be very low
		var sum, sumSquares time.Duration
		for _, duration := range times {
			sum += duration
			sumSquares += duration * duration
		}
		mean := sum / time.Duration(len(times))
		variance := (sumSquares/time.Duration(len(times)) - mean*mean)

		t.Logf("Raw() method mean time: %v", mean)
		t.Logf("Raw() method variance: %v", variance)

		// Variance should be very low, indicating consistent performance
		assert.Less(t, variance, 100*time.Microsecond*time.Microsecond,
			"Raw() method should have consistent performance")
	})

	t.Run("Normalized_method_is_constant_time", func(t *testing.T) {
		// THIS WILL FAIL because Normalized() method doesn't exist yet
		times := make([]time.Duration, 100)
		for i := range 100 {
			start := time.Now()
			_ = repoURL.Normalized()
			times[i] = time.Since(start)
		}

		// All calls should be very fast and consistent (no re-normalization)
		for _, duration := range times {
			assert.Less(t, duration, 100*time.Microsecond,
				"Normalized() call should be very fast (no re-normalization)")
		}

		// Verify consistency
		var sum time.Duration
		for _, duration := range times {
			sum += duration
		}
		mean := sum / time.Duration(len(times))

		t.Logf("Normalized() method mean time: %v", mean)

		// Mean should be very low
		assert.Less(t, mean, 50*time.Microsecond,
			"Normalized() method should be very fast on average")
	})

	t.Run("String_method_is_constant_time", func(t *testing.T) {
		times := make([]time.Duration, 100)
		for i := range 100 {
			start := time.Now()
			_ = repoURL.String()
			times[i] = time.Since(start)
		}

		// All calls should be very fast and consistent
		for _, duration := range times {
			assert.Less(t, duration, 100*time.Microsecond,
				"String() call should be very fast")
		}
	})
}

// BenchmarkRepositoryURL_Creation benchmarks RepositoryURL creation (includes normalization).
func BenchmarkRepositoryURL_Creation(b *testing.B) {
	testURL := "https://GitHub.com/GoLang/Go.git"

	b.ResetTimer()
	for range b.N {
		_, err := NewRepositoryURL(testURL)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRepositoryURL_Raw benchmarks Raw() method calls.
func BenchmarkRepositoryURL_Raw(b *testing.B) {
	repoURL, err := NewRepositoryURL("https://GitHub.com/GoLang/Go.git")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	// THIS WILL FAIL because Raw() method doesn't exist yet
	for range b.N {
		_ = repoURL.Raw()
	}
}

// BenchmarkRepositoryURL_Normalized benchmarks Normalized() method calls.
func BenchmarkRepositoryURL_Normalized(b *testing.B) {
	repoURL, err := NewRepositoryURL("https://GitHub.com/GoLang/Go.git")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	// THIS WILL FAIL because Normalized() method doesn't exist yet
	for range b.N {
		_ = repoURL.Normalized()
	}
}

// BenchmarkRepositoryURL_String benchmarks String() method calls.
func BenchmarkRepositoryURL_String(b *testing.B) {
	repoURL, err := NewRepositoryURL("https://GitHub.com/GoLang/Go.git")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		_ = repoURL.String()
	}
}

// TestRepositoryURL_NormalizationOnlyDuringCreation verifies normalization happens only once.
func TestRepositoryURL_NormalizationOnlyDuringCreation(t *testing.T) {
	// This test documents the expected behavior:
	// 1. Normalization should happen only during NewRepositoryURL()
	// 2. Raw() should return the original input
	// 3. Normalized() should return pre-computed normalized value
	// 4. String() should return pre-computed normalized value

	testCases := []struct {
		name            string
		rawInput        string
		expectedRaw     string
		expectedNorm    string
		expectDifferent bool
	}{
		{
			name:            "trailing slash normalization",
			rawInput:        "https://github.com/golang/go/",
			expectedRaw:     "https://github.com/golang/go/",
			expectedNorm:    "https://github.com/golang/go",
			expectDifferent: true,
		},
		{
			name:            "git suffix normalization",
			rawInput:        "https://github.com/golang/go.git",
			expectedRaw:     "https://github.com/golang/go.git",
			expectedNorm:    "https://github.com/golang/go",
			expectDifferent: true,
		},
		{
			name:            "mixed case normalization",
			rawInput:        "https://GitHub.com/GoLang/Go",
			expectedRaw:     "https://GitHub.com/GoLang/Go",
			expectedNorm:    "https://github.com/golang/go",
			expectDifferent: true,
		},
		{
			name:            "already normalized input",
			rawInput:        "https://github.com/golang/go",
			expectedRaw:     "https://github.com/golang/go",
			expectedNorm:    "https://github.com/golang/go",
			expectDifferent: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create RepositoryURL - normalization happens here
			repoURL, err := NewRepositoryURL(tc.rawInput)
			require.NoError(t, err, "Failed to create RepositoryURL")

			// THIS WILL FAIL because Raw() and Normalized() methods don't exist yet
			actualRaw := repoURL.Raw()
			actualNorm := repoURL.Normalized()
			actualString := repoURL.String()

			// Verify Raw() returns original input
			assert.Equal(t, tc.expectedRaw, actualRaw,
				"Raw() should return original input")

			// Verify Normalized() returns normalized form
			assert.Equal(t, tc.expectedNorm, actualNorm,
				"Normalized() should return normalized form")

			// Verify String() returns normalized form (backward compatibility)
			assert.Equal(t, tc.expectedNorm, actualString,
				"String() should return normalized form")

			// Verify Normalized() and String() return same value
			assert.Equal(t, actualNorm, actualString,
				"Normalized() and String() should return same value")

			// Verify difference between raw and normalized when expected
			if tc.expectDifferent {
				assert.NotEqual(t, actualRaw, actualNorm,
					"Raw() and Normalized() should differ for input needing normalization")
			} else {
				assert.Equal(t, actualRaw, actualNorm,
					"Raw() and Normalized() should be equal for already normalized input")
			}
		})
	}
}
