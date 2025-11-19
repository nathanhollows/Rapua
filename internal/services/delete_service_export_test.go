package services

// Export internal functions for testing purposes.
// This file should only be compiled during tests.

var (
	IsUploadedFileForTest    = isUploadedFile
	EscapeLikePatternForTest = escapeLikePattern
)
