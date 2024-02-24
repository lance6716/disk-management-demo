package disk_management_demo

import (
	"encoding/binary"
	"fmt"
)

const (
	unitMetadataSize = 4
	unitSize         = 4 * 1024
	followingNumMask = 1<<27 - 1
	usedMask         = 1 << 27
)

type unit struct {
	// followingNum means the next number of following units are has same "used"
	// state with this unit. Only the lower 27 bits of followingNum are used.
	followingNum uint32
	used         bool
}

func decodeUnit(data []byte) unit {
	l := binary.LittleEndian.Uint32(data)
	return unit{
		followingNum: l & followingNumMask,
		used:         l&usedMask != 0,
	}
}

func encodeUnit(u unit, buf []byte) []byte {
	n := u.followingNum
	if u.used {
		n |= usedMask
	}
	return binary.LittleEndian.AppendUint32(buf, n)
}

type unitModification struct {
	unitIdx uint32
	newUnit unit
}

func splitAndInvertUsedForLeft(
	idx uint32,
	u unit,
	leftCnt uint32,
) []unitModification {
	if leftCnt == 0 {
		panic(fmt.Sprintf("leftCnt == 0. idx: %d, u: %+v", idx, u))
	}
	if leftCnt > u.followingNum {
		panic(fmt.Sprintf(
			"leftCnt > u.followingNum. idx: %d, u: %+v, leftCnt: %d",
			idx, u, leftCnt,
		))
	}

	return []unitModification{
		{
			unitIdx: idx,
			newUnit: unit{followingNum: leftCnt - 1, used: !u.used},
		},
		{
			unitIdx: idx + leftCnt,
			newUnit: unit{followingNum: u.followingNum - leftCnt, used: u.used},
		},
	}
}

func mergeUnits(idx uint32, units []unit) []unitModification {
	if len(units) == 0 {
		panic(fmt.Sprintf("len(units) == 0. idx: %d", idx))
	}

	var (
		used   bool
		ret    = make([]unitModification, len(units))
		curIdx = idx
	)

	for i, u := range units {
		if i == 0 {
			used = u.used
			curIdx += u.followingNum + 1
			continue
		}

		if u.used != used {
			panic(fmt.Sprintf(
				"u.used != used. idx: %d, used: %t, u: %+v",
				idx, used, u,
			))
		}

		ret[i] = unitModification{unitIdx: curIdx, newUnit: unit{}}
		curIdx += u.followingNum + 1
	}

	ret[0] = unitModification{
		unitIdx: idx,
		newUnit: unit{followingNum: curIdx - idx - 1, used: used},
	}
	return ret
}
