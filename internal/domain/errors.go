package domain

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrQueueFull       = errors.New("sync queue is full")
	ErrSourceExpired   = errors.New("source credential is invalid or expired")
	ErrUpstream        = errors.New("upstream service error")
	ErrAccountMismatch = errors.New("imported UID does not match the requested account")
)
