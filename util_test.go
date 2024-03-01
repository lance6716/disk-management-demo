package disk_management_demo

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetHighestOneIdx(t *testing.T) {
	require.EqualValues(t, -1, getHighestOneIdx(0))
	require.EqualValues(t, 0, getHighestOneIdx(1))
	require.EqualValues(t, 1, getHighestOneIdx(3))
	require.EqualValues(t, 7, getHighestOneIdx(128))
	require.EqualValues(t, 7, getHighestOneIdx(129))
}

func TestBytesToUnitCnt(t *testing.T) {
	require.EqualValues(t, 0, byteSizeToUnitCnt(0))
	require.EqualValues(t, 1, byteSizeToUnitCnt(1))
	require.EqualValues(t, 1, byteSizeToUnitCnt(unitSize-1))
	require.EqualValues(t, 1, byteSizeToUnitCnt(unitSize))
}

func TestAllocInBitmap(t *testing.T) {
	bitmap := make([]byte, 3)
	allocInBitmap(bitmap, 0, 1)
	require.Equal(t, byte(0b0000_0001), bitmap[0])
	allocInBitmap(bitmap, 2, 2)
	require.Equal(t, byte(0b0000_1101), bitmap[0])
	allocInBitmap(bitmap, 5, 12)
	require.Equal(t, byte(0b1110_1101), bitmap[0])
	require.Equal(t, byte(0b1111_1111), bitmap[1])
	require.Equal(t, byte(0b0000_0001), bitmap[2])
}

func TestFreeInBitmap(t *testing.T) {
	bitmap := slices.Clone(ones[:])
	freeInBitmap(bitmap, 0, 1)
	require.Equal(t, byte(0b1111_1110), bitmap[0])
	freeInBitmap(bitmap, 2, 2)
	require.Equal(t, byte(0b1111_0010), bitmap[0])
	freeInBitmap(bitmap, 5, 12)
	require.Equal(t, byte(0b0001_0010), bitmap[0])
	require.Equal(t, byte(0b0000_0000), bitmap[1])
	require.Equal(t, byte(0b1111_1110), bitmap[2])
}

func TestFindLeadingZerosCnt(t *testing.T) {
	bitmap := slices.Clone(ones[:])
	bitmap[0] = 0b0001_0010
	bitmap[1] = 0b0000_0001
	bitmap[2] = 0b1111_1110
	bitmap[len(bitmap)-1] = 0

	require.EqualValues(t, 1, findLeadingZerosCnt(bitmap, 0))
	require.EqualValues(t, 0, findLeadingZerosCnt(bitmap, 1))
	require.EqualValues(t, 2, findLeadingZerosCnt(bitmap, 2))
	require.EqualValues(t, 1, findLeadingZerosCnt(bitmap, 3))
	require.EqualValues(t, 3, findLeadingZerosCnt(bitmap, 5))
	require.EqualValues(t, 8, findLeadingZerosCnt(bitmap, 9))
	require.EqualValues(t, 8, findLeadingZerosCnt(bitmap, unit((len(bitmap)-1)*8)))
}

func TestFindTrailingZerosCnt(t *testing.T) {
	bitmap := slices.Clone(ones[:])
	bitmap[0] = 0b0001_0010
	bitmap[1] = 0b0000_0001
	bitmap[2] = 0b1111_1110
	bitmap[len(bitmap)-1] = 0

	require.EqualValues(t, 1, findTrailingZerosCnt(bitmap, 1))
	require.EqualValues(t, 0, findTrailingZerosCnt(bitmap, 2))
	require.EqualValues(t, 1, findTrailingZerosCnt(bitmap, 3))
	require.EqualValues(t, 2, findTrailingZerosCnt(bitmap, 4))
	require.EqualValues(t, 3, findTrailingZerosCnt(bitmap, 8))
	require.EqualValues(t, 8, findTrailingZerosCnt(bitmap, 17))
	require.EqualValues(t, 8, findTrailingZerosCnt(bitmap, unitTotalCnt))
}
