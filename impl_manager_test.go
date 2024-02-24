package disk_management_demo

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type largeFileTestSuite struct {
	suite.Suite
	filePath string
}

func TestLargeFileTestSuite(t *testing.T) {
	suite.Run(t, new(largeFileTestSuite))
}

func (s *largeFileTestSuite) SetupSuite() {
	s.filePath = createFileOfSize(s.T(), metadataSize)
}

func createFileOfSize(t *testing.T, size int64) string {
	dir := t.TempDir()
	filePath := path.Join(dir, fmt.Sprintf("test_image_%d", size))

	f, err := os.Create(filePath)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, os.Truncate(filePath, size))
	return filePath
}

func (s *largeFileTestSuite) TestInit() {
	filePath := createFileOfSize(s.T(), 1024)
	m, err := newDiskManagerImpl(filePath)
	require.ErrorContains(s.T(), err, "file size is too small")
	require.Nil(s.T(), m)

	emptyFilePath := createFileOfSize(s.T(), metadataSize)
	m, err = newDiskManagerImpl(emptyFilePath)
	require.NoError(s.T(), err)
	require.EqualValues(s.T(), 0, m.(*diskManagerImpl).freeUnitIdx)
	require.NoError(s.T(), m.Close())

	f, err := os.Open(emptyFilePath)
	require.NoError(s.T(), err)
	var extraUnitBuf [4]byte
	_, err = f.Read(extraUnitBuf[:])
	require.NoError(s.T(), err)
	require.Equal(s.T(), magic, extraUnitBuf[0])
	var buf [4]byte
	_, err = f.Read(buf[:])
	require.NoError(s.T(), err)
	firstUnit := decodeUnit(buf[:])
	require.Equal(s.T(), unit{followingNum: followingNumMask, used: false}, firstUnit)
	require.NoError(s.T(), f.Close())

	m, err = newDiskManagerImpl(emptyFilePath)
	require.NoError(s.T(), err)
	require.EqualValues(s.T(), 0, m.(*diskManagerImpl).freeUnitIdx)
	require.NoError(s.T(), m.Close())
}

func (s *largeFileTestSuite) TestSpaceFull() {
	m, err := newDiskManagerImpl(s.filePath)
	require.NoError(s.T(), err)

	_, err = m.Alloc(1<<40 + 1)
	require.ErrorIs(s.T(), err, ErrNoEnoughSpace)
	require.NoError(s.T(), m.Close())
	// TODO(lance6716): more test
}
