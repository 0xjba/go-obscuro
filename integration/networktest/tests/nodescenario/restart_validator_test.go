package nodescenario

import (
	"testing"
	"time"

	"github.com/obscuronet/go-obscuro/integration/networktest/actions"

	"github.com/obscuronet/go-obscuro/integration/networktest"
	"github.com/obscuronet/go-obscuro/integration/networktest/env"
)

// restart both the host and the enclave for a validator
func TestRestartValidatorNode(t *testing.T) {
	networktest.TestOnlyRunsInIDE(t)
	networktest.Run(
		"restart-node",
		t,
		env.LocalDevNetwork(),
		actions.Series(
			actions.CreateAndFundTestUsers(5),

			// short load test, build up some state
			actions.GenerateUsersRandomisedTransferActionsInParallel(4, 10*time.Second),

			// todo (@matt) - this could be replaced by something that finds all the transaction IDs in context and waits for them to be mined
			actions.SleepAction(5*time.Second), // allow time for in-flight transactions

			// restart host and enclave on a validator
			actions.StopValidatorEnclave(1),
			actions.StopValidatorHost(1),
			actions.SleepAction(5*time.Second), // allow time for shutdown
			actions.StartValidatorEnclave(1),
			actions.StartValidatorHost(1),
			actions.WaitForValidatorHealthCheck(1, 30*time.Second),

			// todo (@matt) - we often see 1 transaction getting lost without this sleep after the node restarts.
			// 	This needs investigating but it suggests to me that the health check is succeeding prematurely
			actions.SleepAction(5*time.Second), // allow time for re-sync

			// resubmit user viewing keys (any users attached to the restarted node will have lost their "session")
			// todo (@matt) - get rid of this once the enclave persists viewing keys correctly
			actions.AuthenticateAllUsers(),

			// another load test (important that at least one of the users will be using the validator with restarted enclave)
			actions.GenerateUsersRandomisedTransferActionsInParallel(4, 10*time.Second),
		),
	)
}
