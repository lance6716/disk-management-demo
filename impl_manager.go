package disk_management_demo

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
)

const (
	metadataSize       = 1024*1024*1024 + 4
	magic         byte = 0x12
	extraUnitSize      = 4
)

type diskManagerImpl struct {
	f *os.File
	// freeUnitIdx always point to a free unit, or -1 if there is no free unit.
	freeUnitIdx int32
}

func newDiskManagerImpl(imageFilePath string) (Manager, error) {
	f, err := os.OpenFile(imageFilePath, os.O_RDWR, 0644)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if s := stat.Size(); s < metadataSize {
		return nil, errors.Errorf("file size is too small: %d", s)
	}

	m := &diskManagerImpl{
		f:           f,
		freeUnitIdx: -1,
	}
	return m, m.initFreeUnitIdx()
}

// initFreeUnitIdx is used when the disk manager is created. It initializes the
// freeUnitIdx field by scanning the metadata.
//
// pre-condition: d.f is opened and the size is enough for the metadata.
//
// post-condition: d.freeUnitIdx is set to the index of the first free unit, or
// -1 if there is no free unit.
func (d *diskManagerImpl) initFreeUnitIdx() error {
	// first check if this is a fresh new file

	var extra [extraUnitSize]byte
	_, err := d.f.ReadAt(extra[:], 0)
	if err != nil {
		return errors.WithStack(err)
	}
	if extra[0] != magic {
		// this branch means the file is a fresh new file, initialize the metadata and
		// then write the magic.
		mod := unitModification{
			unitIdx: 0,
			newUnit: unit{followingNum: followingNumMask, used: false},
		}
		if err = d.commitModification([]unitModification{mod}); err != nil {
			return err
		}
		d.freeUnitIdx = 0
		if _, err = d.f.WriteAt([]byte{magic}, 0); err != nil {
			return errors.WithStack(err)
		}
		return nil
	}

	freeUnitIdx, _, err := d.findEnoughFreeUnits(0, 1)
	if errors.Is(err, ErrNoEnoughSpace) {
		// ignore this error, it's fine to have no free unit because later caller may
		// release some.
		d.freeUnitIdx = -1
		return nil
	}
	d.freeUnitIdx = int32(freeUnitIdx)
	return err
}

func (d *diskManagerImpl) commitModification(ms []unitModification) error {
	// TODO(lance6716): mergeFollowingUnits!!
	var buf [4]byte
	// TODO: need mechanism like WAL to ensure the modification is atomic
	for _, m := range ms {
		_, err := d.f.WriteAt(
			encodeUnit(m.newUnit, buf[:0]),
			unitIdxToOffset(m.unitIdx),
		)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func unitIdxToOffset(unitIdx uint32) int64 {
	return int64(extraUnitSize + unitIdx*unitSize)
}

func (d *diskManagerImpl) findEnoughFreeUnits(startIdx, n uint32) (uint32, unit, error) {
	if n > followingNumMask+1 {
		return 0, unit{}, ErrNoEnoughSpace
	}
	curIdx := startIdx
	var buf [unitMetadataSize]byte

	for {
		_, err := d.f.ReadAt(buf[:], unitIdxToOffset(curIdx))
		if err != nil {
			return 0, unit{}, errors.WithStack(err)
		}
		u := decodeUnit(buf[:unitMetadataSize])
		if !u.used && u.followingNum >= n {
			return curIdx, u, nil
		}

		curIdx += u.followingNum + 1
		if curIdx == followingNumMask+1 {
			curIdx = 0
		}
		if curIdx == startIdx {
			return 0, unit{}, ErrNoEnoughSpace
		}
	}
}

// Alloc implements Manager.Alloc.
func (d *diskManagerImpl) Alloc(size int64) (startOffset int64, _ error) {
	if d.freeUnitIdx < 0 {
		return 0, ErrNoEnoughSpace
	}
	// ceiling division
	unitCnt := uint32((size-1)/unitSize + 1)

	startIdx, startUnit, err := d.findEnoughFreeUnits(uint32(d.freeUnitIdx), unitCnt)
	if err != nil {
		return 0, err
	}
	startOffset = int64(startIdx) * unitSize

	switch {
	case unitCnt < startUnit.followingNum+1:
		modifies := splitAndInvertUsedForLeft(startIdx, startUnit, unitCnt)
		if err = d.commitModification(modifies); err != nil {
			return 0, err
		}
		d.freeUnitIdx = int32(startIdx + unitCnt)
		return startOffset, nil

	case unitCnt == startUnit.followingNum+1:
		startUnit.used = true
		modifies, freeUnitIdx, err := d.mergeFollowingUnits(startIdx, startUnit)
		if err != nil {
			return 0, err
		}
		if err = d.commitModification(modifies); err != nil {
			return 0, err
		}
		d.freeUnitIdx = int32(freeUnitIdx)
		return startOffset, nil

	default:
		panic(fmt.Sprintf(
			"findEnoughFreeUnits has no error but space is not enough. unitCnt: %d, startIdx: %d, startUnit: %+v",
			unitCnt, startIdx, startUnit,
		))
	}
}

func (d *diskManagerImpl) mergeFollowingUnits(
	idx uint32,
	u unit,
) (_ []unitModification, nextUnitIdx uint32, _ error) {
	nextUnitIdx = idx + u.followingNum + 1
	if nextUnitIdx == followingNumMask+1 {
		return []unitModification{{
			unitIdx: idx,
			newUnit: u,
		}}, 0, nil
	}

	if nextUnitIdx > followingNumMask+1 {
		panic(fmt.Sprintf(
			"nextUnitIdx > followingNumMask+1. idx: %d, u: %+v",
			idx, u,
		))
	}

	var (
		buf          [unitMetadataSize]byte
		toMergeUnits = make([]unit, 0, 1)
	)
	toMergeUnits = append(toMergeUnits, u)

	for nextUnitIdx <= followingNumMask {
		_, err := d.f.ReadAt(buf[:], unitIdxToOffset(nextUnitIdx))
		if err != nil {
			return nil, 0, errors.WithStack(err)
		}
		nextUnit := decodeUnit(buf[:unitMetadataSize])

		if nextUnit.used != u.used {
			break
		}

		toMergeUnits = append(toMergeUnits, nextUnit)
		nextUnitIdx += nextUnit.followingNum + 1
	}

	if nextUnitIdx > followingNumMask+1 {
		panic(fmt.Sprintf(
			"nextUnitIdx > followingNumMask+1. idx: %d, toMergeUnits: %+v",
			idx, toMergeUnits,
		))
	}
	if nextUnitIdx == followingNumMask+1 {
		nextUnitIdx = 0
	}

	return mergeUnits(idx, toMergeUnits), nextUnitIdx, nil
}

func (d *diskManagerImpl) Free(startOffset int64, size int64) error {
	//TODO implement me
	panic("implement me")
}

func (d *diskManagerImpl) Close() error {
	return d.f.Close()
}
