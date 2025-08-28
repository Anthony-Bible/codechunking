package valueobject

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// BinaryFileSignature represents a file signature (magic bytes) used for binary file detection.
// It serves as a value object in the domain layer for identifying file types based on
// their binary content headers.
type BinaryFileSignature struct {
	name        string
	description string
	magicBytes  []byte
	offset      int
	fileType    string
	extensions  []string
	priority    int
	enabled     bool
}

// NewBinaryFileSignature creates a new BinaryFileSignature value object with validation.
func NewBinaryFileSignature(name, hexSignature string, offset int, fileType string) (BinaryFileSignature, error) {
	if name == "" {
		return BinaryFileSignature{}, errors.New("signature name cannot be empty")
	}

	if hexSignature == "" {
		return BinaryFileSignature{}, errors.New("hex signature cannot be empty")
	}

	if fileType == "" {
		return BinaryFileSignature{}, errors.New("file type cannot be empty")
	}

	// Normalize name
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return BinaryFileSignature{}, errors.New("signature name cannot be empty after normalization")
	}

	// Validate and convert hex signature
	magicBytes, err := validateAndConvertHexSignature(hexSignature)
	if err != nil {
		return BinaryFileSignature{}, fmt.Errorf("invalid hex signature: %w", err)
	}

	// Validate offset
	if offset < 0 {
		return BinaryFileSignature{}, errors.New("offset cannot be negative")
	}

	// Validate file type
	normalizedFileType := strings.TrimSpace(fileType)
	if normalizedFileType == "" {
		return BinaryFileSignature{}, errors.New("file type cannot be empty after normalization")
	}

	return BinaryFileSignature{
		name:        normalizedName,
		description: "",
		magicBytes:  magicBytes,
		offset:      offset,
		fileType:    normalizedFileType,
		extensions:  []string{},
		priority:    0,
		enabled:     true,
	}, nil
}

// NewBinaryFileSignatureWithDetails creates a BinaryFileSignature with comprehensive details.
func NewBinaryFileSignatureWithDetails(
	name, hexSignature, description, fileType string,
	offset, priority int,
	extensions []string,
	enabled bool,
) (BinaryFileSignature, error) {
	// Create base signature
	signature, err := NewBinaryFileSignature(name, hexSignature, offset, fileType)
	if err != nil {
		return BinaryFileSignature{}, err
	}

	// Validate description
	if len(description) > 500 {
		return BinaryFileSignature{}, errors.New("description too long (max 500 characters)")
	}

	// Validate priority
	if priority < 0 || priority > 1000 {
		return BinaryFileSignature{}, errors.New("priority must be between 0 and 1000")
	}

	// Validate extensions
	validatedExtensions, err := validateFileExtensions(extensions)
	if err != nil {
		return BinaryFileSignature{}, fmt.Errorf("invalid extensions: %w", err)
	}

	signature.description = strings.TrimSpace(description)
	signature.priority = priority
	signature.extensions = validatedExtensions
	signature.enabled = enabled

	return signature, nil
}

// validateAndConvertHexSignature validates and converts hex string to bytes.
func validateAndConvertHexSignature(hexSignature string) ([]byte, error) {
	// Remove spaces and normalize
	cleaned := strings.ReplaceAll(strings.ToUpper(hexSignature), " ", "")

	// Remove common prefixes
	cleaned = strings.TrimPrefix(cleaned, "0X")
	cleaned = strings.TrimPrefix(cleaned, "\\X")

	// Validate hex characters
	if len(cleaned)%2 != 0 {
		return nil, errors.New("hex signature must have even number of characters")
	}

	if len(cleaned) == 0 {
		return nil, errors.New("hex signature cannot be empty")
	}

	if len(cleaned) > 64 {
		return nil, errors.New("hex signature too long (max 32 bytes)")
	}

	// Validate hex characters
	for _, char := range cleaned {
		if (char < '0' || char > '9') && (char < 'A' || char > 'F') {
			return nil, fmt.Errorf("invalid hex character: %c", char)
		}
	}

	// Convert to bytes
	magicBytes, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex signature: %w", err)
	}

	return magicBytes, nil
}

// validateFileExtensions validates a list of file extensions.
func validateFileExtensions(extensions []string) ([]string, error) {
	if len(extensions) > 20 {
		return nil, errors.New("too many extensions (max 20)")
	}

	validated := make([]string, 0, len(extensions))
	seen := make(map[string]bool)

	for _, ext := range extensions {
		trimmed := strings.TrimSpace(ext)
		if trimmed == "" {
			continue
		}

		// Ensure extension starts with a dot
		if !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + trimmed
		}

		// Convert to lowercase for consistency
		trimmed = strings.ToLower(trimmed)

		// Check for duplicates
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true

		// Validate extension format
		if err := validateBinaryExtension(trimmed); err != nil {
			return nil, fmt.Errorf("invalid extension %s: %w", trimmed, err)
		}

		validated = append(validated, trimmed)
	}

	return validated, nil
}

// validateBinaryExtension validates a single file extension for binary files.
func validateBinaryExtension(ext string) error {
	if len(ext) < 2 { // Must be at least "."
		return errors.New("extension too short")
	}

	if !strings.HasPrefix(ext, ".") {
		return errors.New("extension must start with a dot")
	}

	// Check for valid characters (alphanumeric, dash, underscore)
	for i, char := range ext[1:] { // Skip the dot
		if (char < 'a' || char > 'z') &&
			(char < '0' || char > '9') &&
			char != '-' && char != '_' {
			return fmt.Errorf("invalid character at position %d: %c", i+1, char)
		}
	}

	return nil
}

// Name returns the signature name.
func (s BinaryFileSignature) Name() string {
	return s.name
}

// Description returns the signature description.
func (s BinaryFileSignature) Description() string {
	return s.description
}

// MagicBytes returns a copy of the magic bytes.
func (s BinaryFileSignature) MagicBytes() []byte {
	if s.magicBytes == nil {
		return nil
	}
	result := make([]byte, len(s.magicBytes))
	copy(result, s.magicBytes)
	return result
}

// Offset returns the offset where magic bytes should be found.
func (s BinaryFileSignature) Offset() int {
	return s.offset
}

// FileType returns the file type identifier.
func (s BinaryFileSignature) FileType() string {
	return s.fileType
}

// Extensions returns a copy of the file extensions.
func (s BinaryFileSignature) Extensions() []string {
	if s.extensions == nil {
		return []string{}
	}
	result := make([]string, len(s.extensions))
	copy(result, s.extensions)
	return result
}

// Priority returns the signature priority.
func (s BinaryFileSignature) Priority() int {
	return s.priority
}

// IsEnabled returns true if the signature is enabled.
func (s BinaryFileSignature) IsEnabled() bool {
	return s.enabled
}

// HexSignature returns the magic bytes as a hex string.
func (s BinaryFileSignature) HexSignature() string {
	if s.magicBytes == nil {
		return ""
	}
	return strings.ToUpper(hex.EncodeToString(s.magicBytes))
}

// Matches checks if the given content matches this signature.
func (s BinaryFileSignature) Matches(content []byte) bool {
	if !s.enabled || len(content) == 0 || len(s.magicBytes) == 0 {
		return false
	}

	// Check if content is long enough
	if len(content) < s.offset+len(s.magicBytes) {
		return false
	}

	// Extract the relevant portion of content
	contentSection := content[s.offset : s.offset+len(s.magicBytes)]

	// Compare magic bytes
	return bytes.Equal(contentSection, s.magicBytes)
}

// MatchesFile checks if a file path and content match this signature.
func (s BinaryFileSignature) MatchesFile(filePath string, content []byte) bool {
	// Check content first
	if !s.Matches(content) {
		return false
	}

	// If no specific extensions are defined, content match is sufficient
	if len(s.extensions) == 0 {
		return true
	}

	// Check if file extension matches any of the signature's extensions
	for _, ext := range s.extensions {
		if strings.HasSuffix(strings.ToLower(filePath), ext) {
			return true
		}
	}

	return false
}

// HasExtension returns true if the signature supports the given extension.
func (s BinaryFileSignature) HasExtension(extension string) bool {
	normalized := strings.ToLower(strings.TrimSpace(extension))
	if !strings.HasPrefix(normalized, ".") {
		normalized = "." + normalized
	}

	for _, ext := range s.extensions {
		if ext == normalized {
			return true
		}
	}
	return false
}

// WithPriority returns a new BinaryFileSignature with updated priority.
func (s BinaryFileSignature) WithPriority(priority int) (BinaryFileSignature, error) {
	if priority < 0 || priority > 1000 {
		return BinaryFileSignature{}, errors.New("priority must be between 0 and 1000")
	}

	newSignature := s
	newSignature.priority = priority
	return newSignature, nil
}

// WithEnabled returns a new BinaryFileSignature with updated enabled status.
func (s BinaryFileSignature) WithEnabled(enabled bool) BinaryFileSignature {
	newSignature := s
	newSignature.enabled = enabled
	return newSignature
}

// WithDescription returns a new BinaryFileSignature with updated description.
func (s BinaryFileSignature) WithDescription(description string) (BinaryFileSignature, error) {
	if len(description) > 500 {
		return BinaryFileSignature{}, errors.New("description too long (max 500 characters)")
	}

	newSignature := s
	newSignature.description = strings.TrimSpace(description)
	return newSignature, nil
}

// WithExtensions returns a new BinaryFileSignature with updated extensions.
func (s BinaryFileSignature) WithExtensions(extensions []string) (BinaryFileSignature, error) {
	validatedExtensions, err := validateFileExtensions(extensions)
	if err != nil {
		return BinaryFileSignature{}, err
	}

	newSignature := s
	newSignature.extensions = validatedExtensions
	return newSignature, nil
}

// String returns a string representation of the binary file signature.
func (s BinaryFileSignature) String() string {
	return fmt.Sprintf("%s (%s): %s at offset %d",
		s.name, s.fileType, s.HexSignature(), s.offset)
}

// Equal compares two BinaryFileSignature instances for equality.
func (s BinaryFileSignature) Equal(other BinaryFileSignature) bool {
	return s.name == other.name &&
		s.fileType == other.fileType &&
		s.offset == other.offset &&
		bytes.Equal(s.magicBytes, other.magicBytes)
}

// GetCommonBinarySignatures returns a list of common binary file signatures.
func GetCommonBinarySignatures() []BinaryFileSignature {
	signatures := []BinaryFileSignature{}

	// PNG files
	if png, err := NewBinaryFileSignatureWithDetails(
		"PNG Image", "89504E47", "Portable Network Graphics image",
		"image", 0, 100, []string{".png"}, true,
	); err == nil {
		signatures = append(signatures, png)
	}

	// JPEG files
	if jpeg, err := NewBinaryFileSignatureWithDetails(
		"JPEG Image", "FFD8FF", "JPEG image file",
		"image", 0, 100, []string{".jpg", ".jpeg"}, true,
	); err == nil {
		signatures = append(signatures, jpeg)
	}

	// GIF files
	if gif, err := NewBinaryFileSignatureWithDetails(
		"GIF Image", "474946383961", "GIF89a image file",
		"image", 0, 100, []string{".gif"}, true,
	); err == nil {
		signatures = append(signatures, gif)
	}

	// PDF files
	if pdf, err := NewBinaryFileSignatureWithDetails(
		"PDF Document", "255044462D", "Portable Document Format",
		"document", 0, 100, []string{".pdf"}, true,
	); err == nil {
		signatures = append(signatures, pdf)
	}

	// ZIP files
	if zip, err := NewBinaryFileSignatureWithDetails(
		"ZIP Archive", "504B0304", "ZIP archive file",
		"archive", 0, 100, []string{".zip", ".jar", ".war", ".ear"}, true,
	); err == nil {
		signatures = append(signatures, zip)
	}

	// ELF executables
	if elf, err := NewBinaryFileSignatureWithDetails(
		"ELF Executable", "7F454C46", "Executable and Linkable Format",
		"executable", 0, 100, []string{".so", ".bin"}, true,
	); err == nil {
		signatures = append(signatures, elf)
	}

	// PE executables
	if pe, err := NewBinaryFileSignatureWithDetails(
		"PE Executable", "4D5A", "Portable Executable (Windows)",
		"executable", 0, 100, []string{".exe", ".dll"}, true,
	); err == nil {
		signatures = append(signatures, pe)
	}

	return signatures
}
