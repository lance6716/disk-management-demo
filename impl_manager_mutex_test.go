package disk_management_demo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAllocDuration2(t *testing.T) {
	tempFile := createFileWithContent(t, nil)
	m, err := newDiskManagerWithMutexImpl(tempFile)
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
