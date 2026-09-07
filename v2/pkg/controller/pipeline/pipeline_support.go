package pipeline

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
)

func executeRegistration(ctx context.Context, request *Request, registration Registration) (error, bool) {
	for _, guard := range registration.guards {
		allowed, err := guard.Evaluate(ctx, request)
		if err != nil {
			return fmt.Errorf("evaluate guard %q for action %q: %w", guard.Name(), registration.name, err), false
		}

		if !allowed {
			return nil, true
		}
	}

	return registration.action.Execute(ctx, request), false
}

func runPhase(
	ctx context.Context,
	request *Request,
	registrations []Registration,
	stopOnBlocking bool,
	outcome action.ActionError,
) action.ActionError {
	for _, registration := range registrations {
		err, skipped := executeRegistration(ctx, request, registration)
		if skipped {
			continue
		}

		var stop bool

		outcome, stop = outcome.Add(registration.name, err)
		if stop && stopOnBlocking {
			break
		}
	}

	return outcome
}
