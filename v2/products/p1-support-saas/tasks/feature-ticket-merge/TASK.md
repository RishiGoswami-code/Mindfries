# Implement ticket merge

Customers often open several tickets for the same problem. Agents need to
**merge** a duplicate ticket into a primary one so the whole conversation lives
in one place.

## Your task

Implement `Merge` in `internal/support/merge.go`:

```go
func (s *Service) Merge(duplicateID, primaryID string) (Ticket, error)
```

It should:

1. Reject merging a ticket into itself.
2. Return a not-found error if either ticket does not exist.
3. Move every message from the duplicate onto the **primary**, preserving order
   and author.
4. Record the merge with a **system** message on the primary (mentioning the
   duplicate's id) and a system message on the duplicate (mentioning the
   primary's id).
5. **Close** the duplicate.
6. Update both tickets' `UpdatedAt` and return the updated primary.

A stub is provided so the package compiles; replace it with your implementation.

## How to check

```sh
go test ./internal/support/
```

Your implementation is judged against the behaviour above. Keep the rest of the
package's tests green.
