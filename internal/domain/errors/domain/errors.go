// Package domain provides domain-specific error definitions and utilities.
package domain

import "errors"

// Repository-related errors.
var (
	ErrRepositoryNotFound      = errors.New("repository not found")
	ErrRepositoryAlreadyExists = errors.New("repository already exists")
	ErrRepositoryProcessing    = errors.New("repository is currently being processed")
	ErrRepositoryInvalidURL    = errors.New("repository URL is invalid")
	ErrInvalidRepositoryURL    = errors.New("repository URL is invalid")
)

// Job-related errors.
var (
	ErrJobNotFound = errors.New("indexing job not found")
	ErrJobFailed   = errors.New("indexing job failed")
)

// Connector-related errors.
var (
	ErrConnectorNotFound      = errors.New("connector not found")
	ErrConnectorAlreadyExists = errors.New("connector already exists")
	ErrConnectorSyncing       = errors.New("connector is currently syncing")
)

// General domain errors.
var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)
