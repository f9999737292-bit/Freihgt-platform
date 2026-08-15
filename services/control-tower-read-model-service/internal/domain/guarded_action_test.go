package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForbiddenActionsAlwaysDenied(t *testing.T) {
	for _, action := range []string{"CHANGE_ROUTE", "CANCEL_SHIPMENT", "AUTHORIZE_PAYMENT", "ARBITRARY_DRIVER_COMMAND"} {
		require.True(t, IsForbiddenAutomationAction(action))
		require.Error(t, ValidateGuardedActionCode(action))
	}
}

func TestAllowedGuardedActions(t *testing.T) {
	for _, action := range []string{
		ActionRequestDriverDelayReason,
		ActionRequestDriverStatusConfirmation,
		ActionRequestDriverArrivalConfirmation,
		ActionCreateDriverOperationalNotice,
	} {
		require.NoError(t, ValidateGuardedActionCode(action))
		taskType, err := MapToDriverTaskType(action)
		require.NoError(t, err)
		require.NotEmpty(t, taskType)
	}
}

func TestExecutionModeGuardedAutoAllowed(t *testing.T) {
	require.NoError(t, ValidateExecutionMode(ExecutionModeGuardedAuto))
}
