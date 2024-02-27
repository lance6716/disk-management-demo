package disk_management_demo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFreeSpaces(t *testing.T) {
	s := newFreeSpaces()
	require.Len(t, s.buckets, totalBucketCnt)

	b0 := s.buckets[0].(*oneLengthBucket)
	require.EqualValues(t, 1, b0.length)
	b126 := s.buckets[126].(*oneLengthBucket)
	require.EqualValues(t, 127, b126.length)
	b127 := s.buckets[127].(*varLengthBucket)
	require.EqualValues(t, 128, b127.lengthLowerBound)
	b148 := s.buckets[148].(*varLengthBucket)
	require.EqualValues(t, unitTotalCnt, b148.lengthLowerBound)
}

func TestInitFreeSpaces(t *testing.T) {
	s := newFreeSpaces()
	zeros := make([]byte, bitmapSize)
	s.loadFromBitmap(zeros)
	checkBucketsHasExpectedLengthAndLocations(t, s, map[unit][]location{
		unitTotalCnt: {{offset: 0, length: unitTotalCnt}},
	})

	ones := allOnesFn()
	s = newFreeSpaces()
	s.loadFromBitmap(ones[:])
	checkBucketsHasExpectedLengthAndLocations(t, s, nil)

	bitmap := make([]byte, bitmapSize)
	s = newFreeSpaces()
	bitmap[0] = 0b0001_0010
	bitmap[1] = 0b0111_0001
	bitmap[bitmapSize-1] = 0b1000_0000
	s.loadFromBitmap(bitmap)
	checkBucketsHasExpectedLengthAndLocations(t, s, map[unit][]location{
		1:                 {{offset: 0, length: 1}},
		2:                 {{offset: 2, length: 2}},
		3:                 {{offset: 5, length: 3}, {offset: 9, length: 3}},
		128 * 1024 * 1024: {{offset: 15, length: 256*1024*1024 - 16}},
	})
}

func TestBucket(t *testing.T) {
	s := newFreeSpaces()
	require.EqualValues(t, 1, s.getBucket(1).(*oneLengthBucket).length)
	require.EqualValues(t, 127, s.getBucket(127).(*oneLengthBucket).length)
	require.EqualValues(t, 128, s.getBucket(128).(*varLengthBucket).lengthLowerBound)
	require.EqualValues(t, 128, s.getBucket(129).(*varLengthBucket).lengthLowerBound)
	require.EqualValues(t, 256, s.getBucket(256).(*varLengthBucket).lengthLowerBound)
	require.EqualValues(t, unitTotalCnt, s.getBucket(unitTotalCnt).(*varLengthBucket).lengthLowerBound)
}
