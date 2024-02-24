package disk_management_demo

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeEncodeUnit(t *testing.T) {
	cases := []struct {
		data     []byte
		want     unit
		panicMsg string
		want2    []byte
	}{
		{
			nil,
			unit{},
			"runtime error: index out of range [3] with length 0",
			nil,
		},
		{
			[]byte{0, 0, 0, 0},
			unit{},
			"",
			[]byte{0, 0, 0, 0},
		},
		{
			[]byte{0xFF, 0xFF, 0xFF, 0xFF},
			unit{1<<27 - 1, true},
			"",
			[]byte{0xFF, 0xFF, 0xFF, 0b00001111},
		},
		{
			// check top 4 bit is unused
			[]byte{0xFF, 0xFF, 0xFF, 0b00001111},
			unit{1<<27 - 1, true},
			"",
			[]byte{0xFF, 0xFF, 0xFF, 0b00001111},
		},
		{
			[]byte{0xFF, 0xFF, 0xFF, 0b00000111},
			unit{1<<27 - 1, false},
			"",
			[]byte{0xFF, 0xFF, 0xFF, 0b00000111},
		},
		{
			[]byte{0x12, 0x34, 0x56, 0x78},
			unit{0x563412, true},
			"",
			[]byte{0x12, 0x34, 0x56, 0x08},
		},
	}

	for _, c := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					got := fmt.Sprintf("%s", r)
					require.Equal(t, c.panicMsg, got)
				}
			}()
			got := decodeUnit(c.data)
			require.Equal(t, c.want, got)
			got2 := encodeUnit(got, nil)
			require.Equal(t, c.want2, got2)
			got = decodeUnit(got2)
			require.Equal(t, c.want, got)
		}()
	}
}

func TestSplitAndInvertUsedForLeft(t *testing.T) {
	u := unit{followingNum: 1, used: true}
	got := splitAndInvertUsedForLeft(10, u, 1)
	expected := []unitModification{
		{
			unitIdx: 10,
			newUnit: unit{followingNum: 0, used: false},
		},
		{
			unitIdx: 11,
			newUnit: unit{followingNum: 0, used: true},
		},
	}
	require.Equal(t, expected, got)
}

func TestMergeUnits(t *testing.T) {
	u0 := unit{followingNum: 0, used: true} // [10]
	u1 := unit{followingNum: 1, used: true} // [11, 12]

	got := mergeUnits(10, []unit{u0})
	expected := []unitModification{
		{
			unitIdx: 10,
			newUnit: unit{followingNum: 0, used: true},
		},
	}
	require.Equal(t, expected, got)

	got = mergeUnits(10, []unit{u0, u1})
	expected = []unitModification{
		{
			unitIdx: 10,
			newUnit: unit{followingNum: 2, used: true},
		},
		{
			unitIdx: 11,
			newUnit: unit{followingNum: 0, used: false},
		},
	}
	require.Equal(t, expected, got)
}
