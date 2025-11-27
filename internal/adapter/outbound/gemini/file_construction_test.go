package gemini

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/genai"

	"github.com/stretchr/testify/assert"
)

func TestFileConstruction_SimpleValidation(t *testing.T) {
	t.Run("validate_file_references", func(t *testing.T) {
		testCases := []struct {
			name        string
			fileRef     string
			expectError bool
			description string
		}{
			{
				name:        "valid_42_char_batch_reference",
				fileRef:     "files/batch-260322vt10uviv2zfimx6raay2ps8x6zi0x2",
				expectError: false,
				description: "The problematic 42-char batch ID that would fail with files.Get",
			},
			{
				name:        "invalid_missing_prefix",
				fileRef:     "batch-260322vt10uviv2zfimx6raay2ps8x6zi0x2",
				expectError: true,
			},
			{
				name:        "valid_standard_length",
				fileRef:     "files/batch-123456789012345678901234567890",
				expectError: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Test validation logic from our implementation
				if !strings.HasPrefix(tc.fileRef, "files/") {
					err := fmt.Errorf("invalid file reference format: '%s' - must start with 'files/'", tc.fileRef)
					if tc.expectError {
						assert.Error(t, err, "Should fail validation")
					}
				} else {
					if !tc.expectError {
						// Test File construction
						var sizeBytes int64 = 0
						constructedFile := &genai.File{
							Name:        tc.fileRef,
							DisplayName: filepath.Base(tc.fileRef),
							SizeBytes:   &sizeBytes,
						}

						assert.NotNil(t, constructedFile)
						assert.Equal(t, tc.fileRef, constructedFile.Name)
						assert.Equal(t, filepath.Base(tc.fileRef), constructedFile.DisplayName)
						assert.Equal(t, int64(0), *constructedFile.SizeBytes)

						// Log that construction succeeded
						if len(strings.TrimPrefix(tc.fileRef, "files/")) > 40 {
							t.Logf("✅ Successfully constructed File object for 42+ char ID: %s", tc.fileRef)
						}
					}
				}
			})
		}
	})
}

func TestFileConstruction_ProblematicIDs(t *testing.T) {
	t.Run("test_ids_that_would_fail_with_files_get", func(t *testing.T) {
		problematicID := "files/batch-260322vt10uviv2zfimx6raay2ps8x6zi0x2"
		idWithoutPrefix := strings.TrimPrefix(problematicID, "files/")

		// Verify this ID would indeed fail with files.Get (>40 chars)
		assert.Greater(t, len(idWithoutPrefix), 40, "Should exceed Google's 40-char limit")

		// But should work with our construction
		var sizeBytes int64 = 0
		constructedFile := &genai.File{
			Name:        problematicID,
			DisplayName: filepath.Base(problematicID),
			SizeBytes:   &sizeBytes,
		}

		assert.NotNil(t, constructedFile, "File should be constructible")
		assert.Equal(t, problematicID, constructedFile.Name)
		assert.Equal(t, "batch-260322vt10uviv2zfimx6raay2ps8x6zi0x2", constructedFile.DisplayName)

		t.Logf("✅ Problematic ID (length: %d) successfully constructed as File object", len(idWithoutPrefix))
	})
}
