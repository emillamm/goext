package integrationtest

import "context"

type Service interface {
	// Start service async. If startup fails, it will transition to done.
	Start(ctx context.Context)
	// Stop service async. Will be marked as stopped even if it was never started.
	Stop()
	// Waits for service to be ready or done. If it is already done, it should not have an effect calling this method.
	WaitForReady()
	// Waits for service to be done
	WaitForDone()
	// Any error that happened during startup, running or shutdown
	Err() error
}

