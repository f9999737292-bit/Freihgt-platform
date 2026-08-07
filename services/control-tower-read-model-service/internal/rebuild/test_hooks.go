package rebuild

import "fmt"

const (
	FailPointAfterBackup              = failAfterBackup
	FailPointAfterDelete              = failAfterDelete
	FailPointAfterInsert              = failAfterPartialInsert
	FailPointBeforePostValidate       = failBeforePostValidate
	FailPointBeforeActive             = failBeforeActive
	FailPointRollbackAfterState       = failRollbackAfterState
	FailPointRollbackAfterDelete      = failRollbackAfterDelete
	FailPointRollbackAfterInsert      = failRollbackPartialRestore
	FailPointRollbackBeforeValidate   = failRollbackBeforeValidate
	FailPointRollbackBeforeRolledBack = failRollbackBeforeRolledBack
)

// SetActivationFailureHookForTest configures transaction failure injection for integration tests.
func SetActivationFailureHookForTest(point string) {
	if point == "" {
		activationFailureHook = nil
		return
	}
	activationFailureHook = func(current string) error {
		if current == point {
			return fmt.Errorf("injected failure at %s", point)
		}
		return nil
	}
}

// SetActivationPauseHookForTest blocks activation at point until release is closed.
// When entered is non-nil, it is closed once when the pause point is reached.
func SetActivationPauseHookForTest(point string, release chan struct{}, entered chan struct{}) {
	if point == "" {
		activationPauseHook = nil
		return
	}
	activationPauseHook = func(current string) {
		if current == point {
			if entered != nil {
				select {
				case <-entered:
				default:
					close(entered)
				}
			}
			<-release
		}
	}
}
