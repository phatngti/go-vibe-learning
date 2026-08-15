package temporal

import (
	"fmt"

	"github/master_go_cqrs/services/orchestrator/internal/payment"
)

const (
	PaymentWorkflowName        = "PaymentOrchestrationWorkflow"
	PublishCommandActivityName = "PublishPaymentSagaCommand"
	WorkflowStateQuery         = "payment-orchestration-state"
	ProcessEventSignalName     = "payment-process-event"
)

func WorkflowID(transactionID string) string {
	return fmt.Sprintf("payment:%s", transactionID)
}

type WorkflowState struct {
	Snapshot       payment.Snapshot `json:"snapshot"`
	Attempt        int              `json:"attempt"`
	SeenMessageIDs []string         `json:"seen_message_ids,omitempty"`
}
