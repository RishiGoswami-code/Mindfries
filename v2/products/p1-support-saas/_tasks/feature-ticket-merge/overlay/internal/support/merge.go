package support

import "errors"

// Merge folds a duplicate ticket into a primary one. See this task's TASK.md
// for the required behaviour.
//
// TODO(candidate): implement this. The stub exists only so the package compiles.
func (s *Service) Merge(duplicateID, primaryID string) (Ticket, error) {
	return Ticket{}, errors.New("Merge is not implemented yet")
}
