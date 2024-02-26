package disk_management_demo

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"testing"
)

func BenchmarkBitmapCheckContinuousZero(b *testing.B) {
	bitmap := make([]byte, 32*1024*1024) // 32MiB
	allZero := make([]byte, 128)
	offsets := make([]int64, 100)
	for i := range offsets {
		offsets[i] = rand.Int63n(32*1024*1024 - 128)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		offset := offsets[i%100]
		cmp := bytes.Compare(bitmap[offset:offset+128], allZero)
		if cmp != 0 {
			panic("unexpected")
		}
	}
}

func BenchmarkParseToCheck(b *testing.B) {
	membuf := make([]byte, 512*1024*1024)
	offsets := make([]int64, 100)
	for i := range offsets {
		offsets[i] = rand.Int63n(512*1024*1024 - 2)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		offset := offsets[i%100]
		num := binary.LittleEndian.Uint16(membuf[offset : offset+2])
		if num != 0 {
			panic("unexpected")
		}
	}
}

func BenchmarkBitmapWriteContinuousOne(b *testing.B) {
	bitmap := make([]byte, 32*1024*1024) // 32MiB
	allOne := make([]byte, 128)
	for i := range allOne {
		allOne[i] = 0xFF
	}
	offsets := make([]int64, 100)
	for i := range offsets {
		offsets[i] = rand.Int63n(32*1024*1024 - 128)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		offset := offsets[i%100]
		copy(bitmap[offset:offset+128], allOne)
	}
}

func BenchmarkEncodeAndWrite(b *testing.B) {
	membuf := make([]byte, 512*1024*1024)
	content := uint16(1024)
	offsets := make([]int64, 100)
	for i := range offsets {
		offsets[i] = rand.Int63n(512*1024*1024 - 2)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		offset := offsets[i%100]
		binary.LittleEndian.PutUint16(membuf[offset:offset+2], content)
	}
}

func BenchmarkBitmapCheckFailed128(b *testing.B) {
	bitmap := make([]byte, 32*1024*1024) // 32MiB
	noMatch := make([]byte, 128)
	noMatch[127] = 0xFF

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for start := 0; start < 32*1024*1024-128; start += 128 {
			cmp := bytes.Compare(bitmap[start:start+128], noMatch)
			if cmp == 0 {
				panic("unexpected")
			}
		}
	}
}

func BenchmarkBitmapCheckFailed1(b *testing.B) {
	bitmap := make([]byte, 32*1024*1024) // 32MiB
	noMatch := make([]byte, 1)
	noMatch[0] = 0xFF

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for start := 0; start < 32*1024*1024-1; start += 1 {
			cmp := bytes.Compare(bitmap[start:start+1], noMatch)
			if cmp == 0 {
				panic("unexpected")
			}
		}
	}
}

type elem struct {
	offset int32
	length int32
}

func BenchmarkIter1MiElem(b *testing.B) {
	bucketSize := 1024 * 1024

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		elements := make([]elem, bucketSize)
		elements[bucketSize-1].length = 1
		b.StartTimer()

		var toTake int
		for j, e := range elements {
			if e.length > 0 {
				toTake = j
				break
			}
		}
		last := len(elements) - 1
		if toTake != last {
			panic("unexpected")
		}

		elements[toTake] = elements[last]
		elements = elements[:last]
	}
}

func BenchmarkIter128MiElem(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		bucket := make([]uint32, 128*1024*1024)
		bucket[128*1024*1024-1] = 1
		b.StartTimer()

		got := -1
		for j, n := range bucket {
			if n == 1 {
				got = j
				break
			}
		}
		last := len(bucket) - 1
		if got != last {
			panic("unexpected")
		}
		bucket[got] = bucket[last]
		bucket = bucket[:last]
	}
}
