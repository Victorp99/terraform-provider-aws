package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/aws/internal/service/cloudformation/lister"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
)

const (
	// Maximum amount of time to wait for a Change Set to be Created
	ChangeSetCreatedTimeout = 5 * time.Minute
)

func WaitChangeSetCreated(conn *cloudformation.CloudFormation, stackID, changeSetName string) (*cloudformation.DescribeChangeSetOutput, error) {
	stateConf := resource.StateChangeConf{
		Pending: []string{
			cloudformation.ChangeSetStatusCreatePending,
			cloudformation.ChangeSetStatusCreateInProgress,
		},
		Target: []string{
			cloudformation.ChangeSetStatusCreateComplete,
		},
		Timeout: ChangeSetCreatedTimeout,
		Refresh: StatusChangeSet(conn, stackID, changeSetName),
	}
	outputRaw, err := stateConf.WaitForState()
	if err != nil {
		return nil, err
	}

	changeSet, ok := outputRaw.(*cloudformation.DescribeChangeSetOutput)
	if !ok {
		return nil, err
	}
	return changeSet, err
}

const (
	// Default maximum amount of time to wait for a StackSetInstance to be Created
	StackSetInstanceCreatedDefaultTimeout = 30 * time.Minute

	// Default maximum amount of time to wait for a StackSetInstance to be Updated
	StackSetInstanceUpdatedDefaultTimeout = 30 * time.Minute

	// Default maximum amount of time to wait for a StackSetInstance to be Deleted
	StackSetInstanceDeletedDefaultTimeout = 30 * time.Minute

	stackSetOperationDelay = 5 * time.Second
)

const (
	// Default maximum amount of time to wait for a StackSet to be Updated
	StackSetUpdatedDefaultTimeout = 30 * time.Minute
)

func WaitStackSetOperationSucceeded(conn *cloudformation.CloudFormation, stackSetName, operationID string, timeout time.Duration) error {
	stateConf := &resource.StateChangeConf{
		Pending: []string{cloudformation.StackSetOperationStatusRunning},
		Target:  []string{cloudformation.StackSetOperationStatusSucceeded},
		Refresh: StatusStackSetOperation(conn, stackSetName, operationID),
		Timeout: timeout,
		Delay:   stackSetOperationDelay,
	}

	log.Printf("[DEBUG] Waiting for CloudFormation StackSet (%s) operation: %s", stackSetName, operationID)
	_, err := stateConf.WaitForState()

	return err
}

const (
	// Default maximum amount of time to wait for a Stack to be Created
	StackCreatedDefaultTimeout = 30 * time.Minute

	stackCreatedMinTimeout = 1 * time.Second

	// Default maximum amount of time to wait for a Stack to be Updated
	StackUpdatedDefaultTimeout = 30 * time.Minute

	stackUpdatedMinTimeout = 5 * time.Second

	// Default maximum amount of time to wait for a Stack to be Deleted
	StackDeletedDefaultTimeout = 30 * time.Minute

	stackDeletedMinTimeout = 5 * time.Second
)

func WaitStackCreated(conn *cloudformation.CloudFormation, stackID, requestToken string, timeout time.Duration) (*cloudformation.Stack, error) {
	stateConf := resource.StateChangeConf{
		Pending: []string{
			cloudformation.StackStatusCreateInProgress,
			cloudformation.StackStatusDeleteInProgress,
			cloudformation.StackStatusRollbackInProgress,
		},
		Target: []string{
			cloudformation.StackStatusCreateComplete,
			cloudformation.StackStatusCreateFailed,
			cloudformation.StackStatusDeleteComplete,
			cloudformation.StackStatusDeleteFailed,
			cloudformation.StackStatusRollbackComplete,
			cloudformation.StackStatusRollbackFailed,
		},
		Timeout:    timeout,
		MinTimeout: stackCreatedMinTimeout,
		Delay:      10 * time.Second,
		Refresh:    StatusStack(conn, stackID),
	}

	outputRaw, err := stateConf.WaitForState()
	if err != nil {
		return nil, err
	}

	stack, ok := outputRaw.(*cloudformation.Stack)
	if !ok {
		return nil, err
	}

	lastStatus := aws.StringValue(stack.StackStatus)
	switch lastStatus {
	// This will be the case if either disable_rollback is false or on_failure is ROLLBACK
	case cloudformation.StackStatusRollbackComplete, cloudformation.StackStatusRollbackFailed:
		reasons, err := getCloudFormationRollbackReasons(conn, stackID, requestToken)
		if err != nil {
			return stack, fmt.Errorf("failed to create CloudFormation stack, rollback requested (%s). Got an error reading failure information: %w", lastStatus, err)
		}
		return stack, fmt.Errorf("failed to create CloudFormation stack, rollback requested (%s): %q", lastStatus, reasons)

	// This will be the case if on_failure is DELETE
	case cloudformation.StackStatusDeleteComplete, cloudformation.StackStatusDeleteFailed:
		reasons, err := getCloudFormationDeletionReasons(conn, stackID, requestToken)
		if err != nil {
			return stack, fmt.Errorf("failed to create CloudFormation stack, delete requested (%s). Got an error reading failure information: %w", lastStatus, err)
		}

		return stack, fmt.Errorf("failed to create CloudFormation stack, delete requested (%s): %q", lastStatus, reasons)

	// This will be the case if either disable_rollback is true or on_failure is DO_NOTHING
	case cloudformation.StackStatusCreateFailed:
		reasons, err := getCloudFormationFailures(conn, stackID, requestToken)
		if err != nil {
			return stack, fmt.Errorf("failed to create CloudFormation stack (%s). Got an error reading failure information: %w", lastStatus, err)
		}
		return stack, fmt.Errorf("failed to create CloudFormation stack (%s): %q", lastStatus, reasons)
	}

	return stack, nil
}

func WaitStackUpdated(conn *cloudformation.CloudFormation, stackID, requestToken string, timeout time.Duration) (*cloudformation.Stack, error) {
	stateConf := resource.StateChangeConf{
		Pending: []string{
			cloudformation.StackStatusUpdateCompleteCleanupInProgress,
			cloudformation.StackStatusUpdateInProgress,
			cloudformation.StackStatusUpdateRollbackInProgress,
			cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress,
		},
		Target: []string{
			cloudformation.StackStatusCreateComplete,
			cloudformation.StackStatusUpdateComplete,
			cloudformation.StackStatusUpdateRollbackComplete,
			cloudformation.StackStatusUpdateRollbackFailed,
		},
		Timeout:    timeout,
		MinTimeout: stackUpdatedMinTimeout,
		Delay:      10 * time.Second,
		Refresh:    StatusStack(conn, stackID),
	}

	outputRaw, err := stateConf.WaitForState()
	if err != nil {
		return nil, err
	}

	stack, ok := outputRaw.(*cloudformation.Stack)
	if !ok {
		return nil, err
	}

	lastStatus := aws.StringValue(stack.StackStatus)
	if lastStatus == cloudformation.StackStatusUpdateRollbackComplete || lastStatus == cloudformation.StackStatusUpdateRollbackFailed {
		reasons, err := getCloudFormationRollbackReasons(conn, stackID, requestToken)
		if err != nil {
			return stack, fmt.Errorf("failed to update CloudFormation stack (%s). Got an error reading failure information: %w", lastStatus, err)
		}

		return stack, fmt.Errorf("failed to update CloudFormation stack (%s): %q", lastStatus, reasons)
	}

	return stack, nil
}

func WaitStackDeleted(conn *cloudformation.CloudFormation, stackID, requestToken string, timeout time.Duration) (*cloudformation.Stack, error) {
	stateConf := resource.StateChangeConf{
		Pending: []string{
			cloudformation.StackStatusDeleteInProgress,
			cloudformation.StackStatusRollbackInProgress,
		},
		Target: []string{
			cloudformation.StackStatusDeleteComplete,
			cloudformation.StackStatusDeleteFailed,
		},
		Timeout:    timeout,
		MinTimeout: stackDeletedMinTimeout,
		Delay:      10 * time.Second,
		Refresh:    StatusStack(conn, stackID),
	}

	outputRaw, err := stateConf.WaitForState()
	if err != nil {
		return nil, err
	}

	stack, ok := outputRaw.(*cloudformation.Stack)
	if !ok {
		return nil, err
	}

	lastStatus := aws.StringValue(stack.StackStatus)
	if lastStatus == cloudformation.StackStatusDeleteFailed {
		reasons, err := getCloudFormationFailures(conn, stackID, requestToken)
		if err != nil {
			return stack, fmt.Errorf("failed to delete CloudFormation stack (%s). Got an error reading failure information: %w", lastStatus, err)
		}

		return stack, fmt.Errorf("failed to delete CloudFormation stack (%s): %q", lastStatus, reasons)
	}

	return stack, nil
}

const (
	TypeRegistrationTimeout = 5 * time.Minute
)

func WaitTypeRegistrationProgressStatusComplete(ctx context.Context, conn *cloudformation.CloudFormation, registrationToken string) (*cloudformation.DescribeTypeRegistrationOutput, error) {
	stateConf := &resource.StateChangeConf{
		Pending: []string{cloudformation.RegistrationStatusInProgress},
		Target:  []string{cloudformation.RegistrationStatusComplete},
		Refresh: StatusTypeRegistrationProgress(ctx, conn, registrationToken),
		Timeout: TypeRegistrationTimeout,
	}

	outputRaw, err := stateConf.WaitForState()

	if output, ok := outputRaw.(*cloudformation.DescribeTypeRegistrationOutput); ok {
		if err != nil && output != nil {
			newErr := errors.New(aws.StringValue(output.Description))

			var te *resource.TimeoutError
			var use *resource.UnexpectedStateError
			if ok := errors.As(err, &te); ok && te.LastError == nil {
				te.LastError = newErr
			} else if ok := errors.As(err, &use); ok && use.LastError == nil {
				use.LastError = newErr
			}
		}

		return output, err
	}

	return nil, err
}

func getCloudFormationDeletionReasons(conn *cloudformation.CloudFormation, stackID, requestToken string) ([]string, error) {
	var failures []string

	err := lister.listStackEventsForOperation(conn, stackID, requestToken, func(e *cloudformation.StackEvent) {
		if isFailedEvent(e) || isStackDeletionEvent(e) {
			failures = append(failures, aws.StringValue(e.ResourceStatusReason))
		}
	})
	return failures, err
}

func getCloudFormationRollbackReasons(conn *cloudformation.CloudFormation, stackID, requestToken string) ([]string, error) {
	var failures []string
	err := lister.listStackEventsForOperation(conn, stackID, requestToken, func(e *cloudformation.StackEvent) {
		if isFailedEvent(e) || isRollbackEvent(e) {
			failures = append(failures, aws.StringValue(e.ResourceStatusReason))
		}
	})
	return failures, err
}

func getCloudFormationFailures(conn *cloudformation.CloudFormation, stackID, requestToken string) ([]string, error) {
	var failures []string

	err := lister.listStackEventsForOperation(conn, stackID, requestToken, func(e *cloudformation.StackEvent) {
		if isFailedEvent(e) {
			failures = append(failures, aws.StringValue(e.ResourceStatusReason))
		}
	})
	return failures, err
}

func isFailedEvent(event *cloudformation.StackEvent) bool {
	return strings.HasSuffix(aws.StringValue(event.ResourceStatus), "_FAILED") && event.ResourceStatusReason != nil
}

func isRollbackEvent(event *cloudformation.StackEvent) bool {
	return strings.HasPrefix(aws.StringValue(event.ResourceStatus), "ROLLBACK_") && event.ResourceStatusReason != nil
}

func isStackDeletionEvent(event *cloudformation.StackEvent) bool {
	return aws.StringValue(event.ResourceStatus) == cloudformation.ResourceStatusDeleteInProgress &&
		aws.StringValue(event.ResourceType) == "AWS::CloudFormation::Stack" &&
		event.ResourceStatusReason != nil
}
