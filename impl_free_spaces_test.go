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

func TestBucket(t *testing.T) {
	s := newFreeSpaces()
	require.EqualValues(t, 1, s.getBucket(1).(*oneLengthBucket).length)
	require.EqualValues(t, 127, s.getBucket(127).(*oneLengthBucket).length)
	require.EqualValues(t, 128, s.getBucket(128).(*varLengthBucket).lengthLowerBound)
	require.EqualValues(t, 128, s.getBucket(129).(*varLengthBucket).lengthLowerBound)
	require.EqualValues(t, 256, s.getBucket(256).(*varLengthBucket).lengthLowerBound)
	require.EqualValues(t, unitTotalCnt, s.getBucket(unitTotalCnt).(*varLengthBucket).lengthLowerBound)
}
