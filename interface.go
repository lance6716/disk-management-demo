package disk_management_demo

type SpaceHandle struct{}

type Manager interface {
	// Write persists the data to the storage and returns a handle of it.
	Write(data []byte) (SpaceHandle, error)
	// Delete removes the data of given handle from the storage.
	Delete(handle SpaceHandle) error
}
