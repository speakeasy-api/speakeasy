// Package singleton provides a generic implementation of the singleton pattern.
package singleton

import "sync"

type singleton[T any, R any] struct {
	once     sync.Once
	instance R
}

// New creates a singleton factory function.
// Returns a function that will always return the same instance.
func New[R any](constructor func() R) func() R {
	var s singleton[struct{}, R]
	return func() R {
		s.once.Do(func() {
			s.instance = constructor()
		})
		return s.instance
	}
}

// NewWithOpts creates a singleton factory function that accepts a config parameter.
// Returns a function that will always return the same instance.
func NewWithOpts[T any, R any](constructor func(T) R) func(T) R {
	var s singleton[T, R]
	return func(cfg T) R {
		s.once.Do(func() {
			s.instance = constructor(cfg)
		})
		return s.instance
	}
}
