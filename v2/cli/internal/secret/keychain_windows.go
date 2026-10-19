//go:build windows

package secret

import (
	"errors"
	"syscall"
	"unsafe"
)

// Windows Credential Manager, through advapi32.

var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	errorNotFound           = syscall.Errno(1168) // ERROR_NOT_FOUND
)

type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

type winCred struct{}

func platformKeychain() Store { return winCred{} }

func (winCred) Name() string { return "keychain" }

func target(id string) (*uint16, error) { return syscall.UTF16PtrFromString(Service + ":" + id) }

func (winCred) Set(id, token string) error {
	t, err := target(id)
	if err != nil {
		return err
	}
	user, _ := syscall.UTF16PtrFromString(id)
	blob := []byte(token)
	c := credential{
		Type:               credTypeGeneric,
		TargetName:         t,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     &blob[0],
		Persist:            credPersistLocalMachine,
		UserName:           user,
	}
	if r, _, e := procCredWrite.Call(uintptr(unsafe.Pointer(&c)), 0); r == 0 {
		return e
	}
	return nil
}

func (winCred) Get(id string) (string, error) {
	t, err := target(id)
	if err != nil {
		return "", err
	}
	var pc *credential
	if r, _, e := procCredRead.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&pc))); r == 0 {
		if errors.Is(e, errorNotFound) {
			return "", ErrNotFound
		}
		return "", e
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pc)))
	b := unsafe.Slice(pc.CredentialBlob, pc.CredentialBlobSize)
	return string(b), nil
}

func (winCred) Delete(id string) error {
	t, err := target(id)
	if err != nil {
		return err
	}
	if r, _, e := procCredDelete.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0); r == 0 && !errors.Is(e, errorNotFound) {
		return e
	}
	return nil
}
