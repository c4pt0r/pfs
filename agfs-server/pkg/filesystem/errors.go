package filesystem

import (
	"errors"
	"fmt"
)

// Standard error types for filesystem operations
// These errors can be checked using errors.Is() for type-safe error handling

var (
	// ErrNotFound indicates a file or directory does not exist
	ErrNotFound = errors.New("not found")

	// ErrPermissionDenied indicates insufficient permissions for the operation
	ErrPermissionDenied = errors.New("permission denied")

	// ErrInvalidArgument indicates an invalid argument was provided
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrAlreadyExists indicates a resource already exists (conflict)
	ErrAlreadyExists = errors.New("already exists")

	// ErrNotDirectory indicates the path is not a directory when one was expected
	ErrNotDirectory = errors.New("not a directory")
)

// NotFoundError represents a file or directory not found error with context
type NotFoundError struct {
	Path string
	Op   string // Operation that failed (e.g., "read", "stat", "readdir")
}

func (e *NotFoundError) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("%s: %s: %s", e.Op, e.Path, "not found")
	}
	return fmt.Sprintf("%s: not found", e.Path)
}

func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// PermissionDeniedError represents a permission error with context
type PermissionDeniedError struct {
	Path   string
	Op     string
	Reason string // Optional reason (e.g., "write-only file")
}

func (e *PermissionDeniedError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("%s: %s: permission denied (%s)", e.Op, e.Path, e.Reason)
	}
	if e.Op != "" {
		return fmt.Sprintf("%s: %s: permission denied", e.Op, e.Path)
	}
	return fmt.Sprintf("%s: permission denied", e.Path)
}

func (e *PermissionDeniedError) Is(target error) bool {
	return target == ErrPermissionDenied
}

// InvalidArgumentError represents an invalid argument error with context
type InvalidArgumentError struct {
	Name   string // Name of the argument
	Value  interface{}
	Reason string
}

func (e *InvalidArgumentError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("invalid argument %s=%v: %s", e.Name, e.Value, e.Reason)
	}
	return fmt.Sprintf("invalid argument %s: %s", e.Name, e.Reason)
}

func (e *InvalidArgumentError) Is(target error) bool {
	return target == ErrInvalidArgument
}

// AlreadyExistsError represents a resource conflict error
type AlreadyExistsError struct {
	Path     string
	Resource string // Type of resource (e.g., "mount", "file", "directory")
}

func (e *AlreadyExistsError) Error() string {
	if e.Resource != "" {
		return fmt.Sprintf("%s already exists: %s", e.Resource, e.Path)
	}
	return fmt.Sprintf("already exists: %s", e.Path)
}

func (e *AlreadyExistsError) Is(target error) bool {
	return target == ErrAlreadyExists
}

// NotDirectoryError represents an error when a directory was expected but the path is not a directory
type NotDirectoryError struct {
	Path string
}

func (e *NotDirectoryError) Error() string {
	return fmt.Sprintf("not a directory: %s", e.Path)
}

func (e *NotDirectoryError) Is(target error) bool {
	return target == ErrNotDirectory
}

// Helper functions to create common errors

// NewNotFoundError creates a new NotFoundError
func NewNotFoundError(op, path string) error {
	return &NotFoundError{Op: op, Path: path}
}

// NewPermissionDeniedError creates a new PermissionDeniedError
func NewPermissionDeniedError(op, path, reason string) error {
	return &PermissionDeniedError{Op: op, Path: path, Reason: reason}
}

// NewInvalidArgumentError creates a new InvalidArgumentError
func NewInvalidArgumentError(name string, value interface{}, reason string) error {
	return &InvalidArgumentError{Name: name, Value: value, Reason: reason}
}

// NewAlreadyExistsError creates a new AlreadyExistsError
func NewAlreadyExistsError(resource, path string) error {
	return &AlreadyExistsError{Resource: resource, Path: path}
}

// NewNotDirectoryError creates a new NotDirectoryError
func NewNotDirectoryError(path string) error {
	return &NotDirectoryError{Path: path}
}
