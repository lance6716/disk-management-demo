package disk_management_demo

import (
	"io"
	"os"

	"github.com/pkg/errors"
)

const (
	metadataSize       = 32 * 1024 * 1024 // 32MiB
	spaceTotalSizeBits = 40
	spaceTotalSize     = 1 << spaceTotalSizeBits // 1TiB
	unitSizeBits       = 12
	unitSize           = 1 << unitSizeBits // 4KiB
	unitTotalCntBits   = spaceTotalSizeBits - unitSizeBits
	unitTotalCnt       = 1 << unitTotalCntBits

	allocLimit = 4 * 1024 * 1024 // 4MiB
)

type diskManagerImpl struct {
	imageFilePath string

	bitmap     [metadataSize]byte
	freeSpaces *freeSpacesTp
}

func newDiskManagerImpl(imageFilePath string) (*diskManagerImpl, error) {
	f, err := os.OpenFile(imageFilePath, os.O_RDWR, 0644)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if s := stat.Size(); s != metadataSize {
		return nil, errors.Errorf("file size is not expected: %d", s)
	}

	m := &diskManagerImpl{freeSpaces: newFreeSpaces(), imageFilePath: imageFilePath}
	if _, err = io.ReadAtLeast(f, m.bitmap[:], metadataSize); err != nil {
		return nil, errors.WithStack(err)
	}
	if err = f.Close(); err != nil {
		return nil, errors.WithStack(err)
	}
	m.initFreeSpaces()
	return m, nil
}

func (d *diskManagerImpl) initFreeSpaces() {
	trailingZeros := uint32(0)
	zerosStartsAt := uint32(0)

	for i, b := range d.bitmap {
		// quick path for 0xFF
		if b == 0xFF {
			if trailingZeros > 0 {
				d.freeSpaces.put(zerosStartsAt, trailingZeros)
				trailingZeros = 0
			}
			continue
		}
		// quick path for 0x00
		if b == 0x00 {
			if trailingZeros == 0 {
				zerosStartsAt = uint32(i) * 8
			}
			trailingZeros += 8
			continue
		}

		for bitIdx := uint32(0); bitIdx < 8; bitIdx++ {
			if b&(1<<bitIdx) != 0 {
				if trailingZeros > 0 {
					d.freeSpaces.put(zerosStartsAt, trailingZeros)
					trailingZeros = 0
				}
			} else {
				if trailingZeros == 0 {
					zerosStartsAt = uint32(i)*8 + bitIdx
				}
				trailingZeros++
			}
		}
	}
	if trailingZeros > 0 {
		d.freeSpaces.put(zerosStartsAt, trailingZeros)
	}
}

// Alloc implements Manager.Alloc.
func (d *diskManagerImpl) Alloc(size int64) (startOffset int64, _ error) {
	if size <= 0 {
		return 0, errors.Errorf("size should be positive, got: %d", size)
	}
	if size > allocLimit {
		return 0, errors.Errorf("size should be less than 4MiB, got: %d", size)
	}
	if size%512*1024 != 0 {
		return 0, errors.Errorf("size should be multiple of 512KiB, got: %d", size)
	}

	unitCnt := bytesToUnitCnt(size)
	unitOffset, length, ok := d.freeSpaces.takeAtLeast(unitCnt)
	if !ok {
		return 0, ErrNoEnoughSpace
	}

	allocInBitmap(d.bitmap[:], unitOffset, unitCnt)
	if length > unitCnt {
		d.freeSpaces.put(unitOffset+unitCnt, length-unitCnt)
	}
	return unitOffsetToBytes(unitOffset), nil
}

// Free implements Manager.Free.
func (d *diskManagerImpl) Free(startOffset int64, size int64) error {
	if startOffset < 0 {
		return errors.Errorf("start offset should be non-negative, got: %d", startOffset)
	}
	if size <= 0 {
		return errors.Errorf("size should be positive, got: %d", size)
	}
	if startOffset+size > spaceTotalSize {
		return errors.Errorf("start offset + size should be less than 1TiB, got: %d", startOffset+size)
	}

	unitOffset := startOffset / unitSize
	unitCnt := bytesToUnitCnt(size)
	freeInBitmap(d.bitmap[:], uint32(unitOffset), unitCnt)

	nextUnitIdx := uint32(unitOffset) + unitCnt
	// TODO(lance6716): search the offset in in freesSpaces
	rightCnt := findLeadingZerosCnt(d.bitmap[:], nextUnitIdx)
	if rightCnt > 0 {
		d.freeSpaces.delete(nextUnitIdx, rightCnt)
	}
	leftCnt := findTrailingZerosCnt(d.bitmap[:], uint32(unitOffset))
	if leftCnt > 0 {
		d.freeSpaces.delete(uint32(unitOffset)-leftCnt, leftCnt)
	}
	d.freeSpaces.put(uint32(unitOffset)-leftCnt, leftCnt+unitCnt+rightCnt)
	return nil
}

func (d *diskManagerImpl) Close() error {
	// TODO(lance6716): atomic write by write to a temp file and rename
	return os.WriteFile(d.imageFilePath, d.bitmap[:], 0600)
}
