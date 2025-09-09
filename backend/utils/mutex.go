package utils

import "sync"

var (
	PendingJobs int
	PendingMu   sync.Mutex
)
