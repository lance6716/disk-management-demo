package disk_management_demo

import "errors"

type SpaceHandle struct {
	// private field
	// TODO
}

var (
	ErrDiskFull      = errors.New("disk is full")
	ErrInvalidHandle = errors.New("invalid handle")
)

type Manager interface {
	// Write persists the data to the storage and returns a handle of it.
	//
	// If the storage is full, it returns ErrDiskFull.
	Write(data []byte) (SpaceHandle, error)
	// Delete removes the data of given handle from the storage.
	//
	// If the handle does not exist, it returns ErrInvalidHandle.
	Delete(handle SpaceHandle) error
}
