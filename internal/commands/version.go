package commands

// GetCurrentVersion is a function variable that will be set by main.go
// This allows all commands to access the current version without circular dependencies
var GetCurrentVersion func() string
