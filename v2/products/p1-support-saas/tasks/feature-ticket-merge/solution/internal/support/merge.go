package support

import "fmt"

// Merge folds a duplicate ticket into a primary one: the duplicate's messages
// move onto the primary, both tickets get a system note recording the merge,
// and the duplicate is closed. Returns the updated primary.
func (s *Service) Merge(duplicateID, primaryID string) (Ticket, error) {
	if duplicateID == primaryID {
		return Ticket{}, fmt.Errorf("cannot merge a ticket into itself")
	}
	dup, err := s.store.GetTicket(duplicateID)
	if err != nil {
		return Ticket{}, err
	}
	primary, err := s.store.GetTicket(primaryID)
	if err != nil {
		return Ticket{}, err
	}

	now := s.now()
	for _, m := range dup.Messages {
		primary.Messages = append(primary.Messages, Message{
			TicketID:   primary.ID,
			AuthorType: m.AuthorType,
			AuthorID:   m.AuthorID,
			Body:       m.Body,
			CreatedAt:  m.CreatedAt,
		})
	}
	primary.Messages = append(primary.Messages, Message{
		TicketID:   primary.ID,
		AuthorType: AuthorSystem,
		Body:       fmt.Sprintf("Merged ticket %s into this ticket.", dup.ID),
		CreatedAt:  now,
	})
	primary.UpdatedAt = now

	dup.Messages = append(dup.Messages, Message{
		TicketID:   dup.ID,
		AuthorType: AuthorSystem,
		Body:       fmt.Sprintf("Merged into %s.", primary.ID),
		CreatedAt:  now,
	})
	dup.Status = StatusClosed
	dup.UpdatedAt = now

	if _, err := s.store.UpdateTicket(dup); err != nil {
		return Ticket{}, err
	}
	return s.store.UpdateTicket(primary)
}
