package disk_management_demo

import "errors"

var (
	ErrNoEnoughSpace = errors.New("no enough space")
	ErrOverflow      = errors.New("offset overflow")
)

// Manager uses a local file to provide a simple disk space allocation management
// interface. All data are persisted in the file.
type Manager interface {
	// Alloc reserves a space of given size and returns the start offset of it.
	//
	// If the storage is full, it returns ErrNoEnoughSpace.
	Alloc(size int64) (startOffset int64, err error)
	// Free releases the space of [startOffset, startOffset+size).
	//
	// If startOffset+size is larger than the size of the storage, it returns
	// ErrOverflow.
	Free(startOffset int64, size int64) error
	Close() error
}

// ManagerConstructor is a function type that creates a Manager. The content of
// Manager is stored in a file specified by imageFilePath.
type ManagerConstructor func(imageFilePath string) (Manager, error)

var NewDiskManager ManagerConstructor = NewDiskManagerImpl
