package disk_management_demo

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"sync"
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

var counter *int64 = new(int64)

type task struct {
	toInc *int64
}

func prepareTasks() ([][]task, int) {
	clientConcurrency := 4000
	taskCntPerClient := 250
	clientsTasks := make([][]task, clientConcurrency)
	for i := range clientsTasks {
		clientsTasks[i] = make([]task, taskCntPerClient)
		for j := range clientsTasks[i] {
			clientsTasks[i][j].toInc = counter
		}
	}
	return clientsTasks, clientConcurrency * taskCntPerClient
}

func BenchmarkCallSerialized(b *testing.B) {
	tasks, expected := prepareTasks()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		*counter = 0
		for _, ts := range tasks {
			for _, t := range ts {
				*t.toInc++
			}
		}
		if *counter != int64(expected) {
			panic("unexpected")
		}
	}
}

type asyncTask struct {
	t task
	r chan int64
}

func prepareAsyncTasks() ([][]asyncTask, int) {
	clientConcurrency := 4000
	taskCntPerClient := 250
	clientsTasks := make([][]asyncTask, clientConcurrency)
	for i := range clientsTasks {
		clientsTasks[i] = make([]asyncTask, taskCntPerClient)
		for j := range clientsTasks[i] {
			clientsTasks[i][j].t.toInc = counter
			clientsTasks[i][j].r = make(chan int64)
		}
	}
	return clientsTasks, clientConcurrency * taskCntPerClient
}

func benchCallWithChan(b *testing.B, ch chan asyncTask) {
	tasks, expected := prepareAsyncTasks()
	var wg sync.WaitGroup
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		*counter = 0
		wg.Add(len(tasks))
		for _, ts := range tasks {
			ts := ts
			go func() {
				for _, t := range ts {
					ch <- t
					<-t.r
				}
				wg.Done()
			}()
		}
		b.StartTimer()

		for j := 0; j < expected; j++ {
			t := <-ch
			*t.t.toInc++
			t.r <- *t.t.toInc
		}
		wg.Wait()

		if *counter != int64(expected) {
			panic("unexpected")
		}
	}
}

func BenchmarkCallChanUnbuffered(b *testing.B) {
	ch := make(chan asyncTask)
	benchCallWithChan(b, ch)
}

func BenchmarkCallChanBuffered(b *testing.B) {
	ch := make(chan asyncTask, 128)
	benchCallWithChan(b, ch)
}

type taskWithLock struct {
	toInc *int64
	mu    *sync.Mutex
}

func prepareTasksWithLock() ([][]taskWithLock, int) {
	clientConcurrency := 4000
	taskCntPerClient := 250
	var mu sync.Mutex
	clientsTasks := make([][]taskWithLock, clientConcurrency)
	for i := range clientsTasks {
		clientsTasks[i] = make([]taskWithLock, taskCntPerClient)
		for j := range clientsTasks[i] {
			clientsTasks[i][j].toInc = counter
			clientsTasks[i][j].mu = &mu
		}
	}
	return clientsTasks, clientConcurrency * taskCntPerClient
}

func BenchmarkCallAcquireLock(b *testing.B) {
	tasks, expected := prepareTasksWithLock()
	var wg sync.WaitGroup
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		*counter = 0
		wg.Add(len(tasks))
		for i := range tasks {
			i := i
			go func() {
				for j := range tasks[i] {
					t := tasks[i][j]
					t.mu.Lock()
					*t.toInc++
					t.mu.Unlock()
				}
				wg.Done()
			}()
		}
		wg.Wait()

		if *counter != int64(expected) {
			panic("unexpected")
		}
	}
}
