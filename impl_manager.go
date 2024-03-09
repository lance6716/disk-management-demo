package disk_management_demo

import (
	"io"
	"os"

	"github.com/pkg/errors"
)

const (
	// predefined constants

	spaceTotalSizeBits = 40
	spaceTotalSize     = 1 << spaceTotalSizeBits // 1TiB
	unitSizeBits       = 12
	unitSize           = 1 << unitSizeBits // 4KiB
	allocLimit         = 4 * 1024 * 1024   // 4MiB

	// calculated constants

	unitTotalCntBits = spaceTotalSizeBits - unitSizeBits
	unitTotalCnt     = 1 << unitTotalCntBits // 256Mi
	bitmapSize       = unitTotalCnt / 8      // 32MiB
)

// unit is the basic allocation unit. It is unitSize bytes. This is a dedicated
// type to avoid confusion with other numbers. It can only be converted using
// byteSizeToUnitCnt, byteOffsetToUnitOffset and unitOffsetToByteOffset.
type unit uint32

// diskManagerImpl implements Manager. It persists a bitmap file to record the
// allocation status of the units. This structure is not thread-safe.
type diskManagerImpl struct {
	imageFilePath string

	bitmap     [bitmapSize]byte
	freeSpaces *freeSpaces
}

func newDiskManagerImpl(imageFilePath string) (*diskManagerImpl, error) {
	f, err := os.OpenFile(imageFilePath, os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if s := stat.Size(); s != bitmapSize {
		return nil, errors.Errorf("file size is not expected: %d", s)
	}

	m := &diskManagerImpl{freeSpaces: newFreeSpaces(), imageFilePath: imageFilePath}
	if _, err = io.ReadAtLeast(f, m.bitmap[:], bitmapSize); err != nil {
		return nil, errors.WithStack(err)
	}
	if err = f.Close(); err != nil {
		return nil, errors.WithStack(err)
	}
	m.freeSpaces.loadFromBitmap(m.bitmap[:])
	m.freeSpaces.rebuildMaxContinuousFree(0)
	return m, nil
}

// Alloc implements Manager.Alloc.
func (d *diskManagerImpl) Alloc(size int64) (offset int64, _ error) {
	if size <= 0 {
		return 0, errors.Errorf("size should be positive, got: %d", size)
	}
	if size > allocLimit {
		return 0, errors.Errorf("size should be less than 4MiB, got: %d", size)
	}
	if size%512 != 0 {
		return 0, errors.Errorf("size should be multiple of 512B, got: %d", size)
	}

	cnt := byteSizeToUnitCnt(size)
	unitOffset, ok := d.freeSpaces.take(cnt)
	if !ok {
		return 0, ErrNoEnoughSpace
	}

	allocInBitmap(d.bitmap[:], unitOffset, cnt)
	return unitOffsetToByteOffset(unitOffset), nil
}

// Free implements Manager.Free.
func (d *diskManagerImpl) Free(offset int64, size int64) error {
	if offset < 0 {
		return errors.Errorf("start offset should be non-negative, got: %d", offset)
	}
	if size <= 0 {
		return errors.Errorf("size should be positive, got: %d", size)
	}
	if offset+size > spaceTotalSize {
		return errors.Errorf("start offset + size should be less than 1TiB, got: %d", offset+size)
	}

	unitOffset := byteOffsetToUnitOffset(offset)
	unitCnt := byteSizeToUnitCnt(size)
	freeInBitmap(d.bitmap[:], unitOffset, unitCnt)

	nextUnitIdx := unitOffset + unitCnt
	// TODO(lance6716): search the offset in in freesSpaces instead of bitmap
	rightCnt := findLeadingZerosCnt(d.bitmap[:], nextUnitIdx)
	if rightCnt > 0 {
		d.freeSpaces.delete(nextUnitIdx, rightCnt)
	}
	leftCnt := findTrailingZerosCnt(d.bitmap[:], unitOffset)
	if leftCnt > 0 {
		d.freeSpaces.delete(unitOffset-leftCnt, leftCnt)
	}
	d.freeSpaces.put(unitOffset-leftCnt, leftCnt+unitCnt+rightCnt)
	return nil
}

func (d *diskManagerImpl) Close() error {
	f, err := os.CreateTemp("", "bitmap")
	if err != nil {
		return errors.WithStack(err)
	}
	if _, err = f.Write(d.bitmap[:]); err != nil {
		return errors.WithStack(err)
	}
	if err = f.Close(); err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(os.Rename(f.Name(), d.imageFilePath))
}
