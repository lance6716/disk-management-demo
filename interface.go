package disk_management_demo

import "errors"

type SpaceHandle struct {
	ID int
	spaceHandlePrivates
}

var (
	ErrDiskFull      = errors.New("disk is full")
	ErrInvalidHandle = errors.New("invalid handle")
)

// Manager uses a local file to provide a simple storage interface for data. All
// data are persisted in the file. Any successful invocation of this interface
// guarantees that the action is persisted.
type Manager interface {
	// Alloc reserves a space of given size and returns a handle of it.
	//
	// If the storage is full, it returns ErrDiskFull.
	Alloc(size int) (SpaceHandle, error)
	// Free releases the space of given handle.
	//
	// If the handle does not exist, it returns ErrInvalidHandle.
	Free(handle SpaceHandle) error
}

// ManagerConstructor is a function type that creates a Manager. The content of
// Manager is stored in a file specified by imageFilePath.
type ManagerConstructor func(imageFilePath string) Manager

var NewDiskManager ManagerConstructor = newDiskManagerImpl
