package disk_management_demo

import "sync"

type diskManager2 struct {
	m  *diskManagerImpl
	mu *sync.Mutex
}

func newDiskManagerWithMutexImpl(imageFilePath string) (*diskManager2, error) {
	m, err := newDiskManagerImpl(imageFilePath)
	if err != nil {
		return nil, err
	}
	return &diskManager2{m: m, mu: &sync.Mutex{}}, nil
}

func NewDiskManagerImpl(imageFilePath string) (Manager, error) {
	return newDiskManagerWithMutexImpl(imageFilePath)
}

func (d *diskManager2) Alloc(size int64) (startOffset int64, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.m.Alloc(size)
}

func (d *diskManager2) Free(startOffset int64, size int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.m.Free(startOffset, size)
}

func (d *diskManager2) Close() error {
	return d.m.Close()
}
