// Package merging provides the core logic for Speakeasy's Round-Trip Engineering (RTE).
//
// It handles the 3-way merge process between:
// 1. Base: The pristine generated code from the previous run (stored in the 'sdk-pristine' shadow branch).
// 2. Current: The user's code on disk, potentially containing manual edits.
// 3. New: The freshly generated code from the current run.
//
// The package coordinates:
// - Retrieving history from Git.
// - Computing 3-way merges to preserve user edits while accepting new generation changes.
// - Detecting and reporting conflicts.
// - Persisting the new generation state to the shadow branch (tree splicing) to support future merges.
package merging
