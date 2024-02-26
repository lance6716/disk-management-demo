package disk_management_demo

import (
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var allOnesFn = func() [metadataSize]byte {
	var ones [metadataSize]byte
	for i := range ones {
		ones[i] = 0xff
	}
	return ones
}

func checkBucketsHasExpectedLengthAndLocations(
	t *testing.T,
	s *freeSpacesTp,
	expected map[uint32][]location,
) {
	if expected == nil {
		expected = make(map[uint32][]location)
	}
	for _, b := range s.buckets {
		switch v := b.(type) {
		case *oneLengthBucket:
			locations, ok := expected[uint32(v.length)]
			if !ok {
				require.Len(t, v.offsets, 0)
				continue
			}

			offsets := make([]uint32, 0, len(locations))
			for _, l := range locations {
				offsets = append(offsets, l.offset)
			}
			require.Equal(t, offsets, v.offsets)
		case *varLengthBucket:
			locations, ok := expected[v.lengthLowerBound]
			if !ok {
				require.Len(t, v.locations, 0)
				continue
			}
			require.Equal(t, locations, v.locations)
		default:
			t.Fatalf("unexpected type: %T", b)
		}
	}
}

func TestInitFreeSpaces(t *testing.T) {
	m := &diskManagerImpl{freeSpaces: newFreeSpaces()}
	m.initFreeSpaces()
	checkBucketsHasExpectedLengthAndLocations(t, m.freeSpaces, map[uint32][]location{
		unitTotalCnt: {{offset: 0, length: unitTotalCnt}},
	})

	m.bitmap = allOnesFn()
	m.freeSpaces = newFreeSpaces()
	m.initFreeSpaces()
	checkBucketsHasExpectedLengthAndLocations(t, m.freeSpaces, nil)

	m.bitmap = [metadataSize]byte{}
	m.freeSpaces = newFreeSpaces()
	m.bitmap[0] = 0b0001_0010
	m.bitmap[1] = 0b0111_0001
	m.bitmap[metadataSize-1] = 0b1000_0000
	m.initFreeSpaces()
	checkBucketsHasExpectedLengthAndLocations(t, m.freeSpaces, map[uint32][]location{
		1:                 {{offset: 0, length: 1}},
		2:                 {{offset: 2, length: 2}},
		3:                 {{offset: 5, length: 3}, {offset: 9, length: 3}},
		128 * 1024 * 1024: {{offset: 15, length: 256*1024*1024 - 16}},
	})
}

func createFileWithContent(t *testing.T, content []byte) string {
	tempFile := path.Join(t.TempDir(), "temp")
	if content == nil {
		content = make([]byte, metadataSize)
	}
	err := os.WriteFile(tempFile, content, 0600)
	require.NoError(t, err)
	return tempFile
}

func TestNewDiskManagerImpl(t *testing.T) {
	_, err := newDiskManagerImpl("not_exist")
	require.ErrorContains(t, err, "no such file or directory")

	_, err = newDiskManagerImpl("README.md")
	require.ErrorContains(t, err, "file size is not expected")

	imageContent := make([]byte, metadataSize)
	imageContent[0] = 0b0001_0010
	tempFile := createFileWithContent(t, imageContent)
	m, err := newDiskManagerImpl(tempFile)
	require.NoError(t, err)
	require.Equal(t, imageContent, m.bitmap[:])
	require.NoError(t, m.Close())
	m, err = newDiskManagerImpl(tempFile)
	require.NoError(t, err)
	require.Equal(t, imageContent, m.bitmap[:])
	require.NoError(t, m.Close())
}

func TestAlloc(t *testing.T) {
	imageContent := make([]byte, metadataSize)
	ones := allOnesFn()
	copy(imageContent, ones[:])

	// first 1024+8 bits is unused
	for i := 0; i < 129; i++ {
		imageContent[i] = 0
	}

	tempFile := createFileWithContent(t, imageContent)
	m, err := newDiskManagerImpl(tempFile)
	require.NoError(t, err)
	_, err = m.Alloc(0)
	require.ErrorContains(t, err, "size should be positive, got: 0")
	_, err = m.Alloc(1)
	require.ErrorContains(t, err, "size should be multiple of 512KiB, got: 1")
	_, err = m.Alloc(1024 * 1024 * 1024)
	require.ErrorContains(t, err, "size should be less than 4MiB, got: 1073741824")

	i := 0
	offset, err := m.Alloc(allocLimit)
	require.NoError(t, err)
	require.EqualValues(t, i, offset)
	i += allocLimit

	for ; i < allocLimit+8*unitSize; i += unitSize {
		offset, err = m.Alloc(unitSize)
		require.NoError(t, err)
		require.EqualValues(t, i, offset)
	}
	_, err = m.Alloc(unitSize)
	require.ErrorIs(t, err, ErrNoEnoughSpace)
	err = m.Close()
	require.NoError(t, err)

	got, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	require.Equal(t, ones[:], got)
}

func TestFree(t *testing.T) {
	tempFile := createFileWithContent(t, nil)
	m, err := newDiskManagerImpl(tempFile)
	require.NoError(t, err)

	// alloc 4KiB, 4MiB, 4KiB at the front

	i := 0
	offset, err := m.Alloc(unitSize)
	require.NoError(t, err)
	require.EqualValues(t, i, offset)
	i += unitSize
	offset2, err := m.Alloc(allocLimit)
	require.NoError(t, err)
	require.EqualValues(t, i, offset2)
	i += allocLimit
	offset3, err := m.Alloc(unitSize)
	require.NoError(t, err)
	require.EqualValues(t, i, offset3)
	i += unitSize
	checkBucketsHasExpectedLengthAndLocations(t, m.freeSpaces, map[uint32][]location{
		128 * 1024 * 1024: {{offset: 1026, length: 256*1024*1024 - 1026}},
	})

	err = m.Free(offset2, allocLimit)
	require.NoError(t, err)
	checkBucketsHasExpectedLengthAndLocations(t, m.freeSpaces, map[uint32][]location{
		1024:              {{offset: 1, length: 1024}},
		128 * 1024 * 1024: {{offset: 1026, length: 256*1024*1024 - 1026}},
	})

	err = m.Free(offset, unitSize)
	require.NoError(t, err)
	checkBucketsHasExpectedLengthAndLocations(t, m.freeSpaces, map[uint32][]location{
		1024:              {{offset: 0, length: 1025}},
		128 * 1024 * 1024: {{offset: 1026, length: 256*1024*1024 - 1026}},
	})

	err = m.Free(offset3, unitSize)
	require.NoError(t, err)
	checkBucketsHasExpectedLengthAndLocations(t, m.freeSpaces, map[uint32][]location{
		unitTotalCnt: {{offset: 0, length: unitTotalCnt}},
	})
}

func TestAllocDuration(t *testing.T) {
	tempFile := createFileWithContent(t, nil)
	m, err := newDiskManagerImpl(tempFile)
	require.NoError(t, err)

	allocSize := 1024 * 1024
	expectedCnt := spaceTotalSize / allocSize

	start := time.Now()
	for i := 0; i < expectedCnt; i++ {
		_, err = m.Alloc(int64(allocSize))
		if err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	_, err = m.Alloc(int64(allocSize))
	require.ErrorIs(t, err, ErrNoEnoughSpace)

	t.Logf("%d Allocs took %s, %s/alloc", expectedCnt, elapsed, elapsed/time.Duration(expectedCnt))
}
