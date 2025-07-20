package services

// services should wrap any error that can come from their process
//    e.i. http errors should be wrapped
//    and database errors need not be wrapped

import "errors"

var (
	// retrying later could work
	ErrTemporaryNetworkFailure = errors.New("network failure")

	// retrying probably wouldn't work, this is checked first so if any
	// error is this the service will be flagged for manual review
	ErrIncorrectAssumption = errors.New("unrecoverable failure")
)
