package fakeapi

// sessionExt holds what later parts of the contract record per session.
type sessionExt struct {
	lastSeq int64
	built   *built
}
