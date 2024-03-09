package disk_management_demo

import (
	"slices"
	"sync"
)

const (
	oneLengthBucketThresholdBits = 7
	oneLengthBucketThreshold     = 1 << oneLengthBucketThresholdBits // 128
	varLengthBucketCnt           = unitTotalCntBits - oneLengthBucketThresholdBits + 1
	totalBucketCnt               = varLengthBucketCnt + oneLengthBucketThreshold - 1
)

// freeSpaces is a structure to query continuous free units. It divides the
// length of continuous free units into several buckets, and forward the
// invocations to the bucket.
type freeSpaces struct {
	buckets [totalBucketCnt]bucket

	maxContinuousFree struct {
		state maxContinuousFreeState
		// below fields are only valid when state is stateValid
		bucket *varLengthBucket
		loc    *location
	}
}

type maxContinuousFreeState int

const (
	// stateNeedRebuild means maxContinuousFree is not valid but should scan the free
	// spaces to rebuild it. It's converted from stateExhausted when a put is called.
	// This is the zero value of maxContinuousFreeState, so it's the initial state.
	stateNeedRebuild maxContinuousFreeState = iota
	// stateValid means maxContinuousFree is valid. It's converted from
	// stateNeedRebuild.
	stateValid
	// stateExhausted means maxContinuousFree is exhausted, all allocations can
	// return ErrNoEnoughSpace now. It's converted from stateValid when the first
	// allocation fails, or from stateNeedRebuild when rebuild fails.
	stateExhausted
)

// newFreeSpaces creates a freeSpaces. The freeSpaces is not ready to use until
// freeSpaces.loadFromBitmap is called.
func newFreeSpaces() *freeSpaces {
	s := &freeSpaces{}
	for i := range s.buckets {
		if i+1 < oneLengthBucketThreshold {
			// buckets[0] has length 1, ... buckets[126] has length 127
			s.buckets[i] = &oneLengthBucket{length: unit(i + 1)}
		} else {
			extraExponent := i + 1 - oneLengthBucketThreshold
			s.buckets[i] = &varLengthBucket{
				lengthLowerBound: oneLengthBucketThreshold * (1 << extraExponent),
			}
		}
	}
	return s
}

// loadFromBitmap loads the continuous free units from the bitmap into the
// freeSpaces.
func (s *freeSpaces) loadFromBitmap(bitmap []byte) {
	zerosCnt := unit(0)
	zerosStartsAt := unit(0)

	for i, b := range bitmap {
		// quick path for 0xFF
		if b == 0xFF {
			if zerosCnt > 0 {
				s.put(zerosStartsAt, zerosCnt)
				zerosCnt = 0
			}
			continue
		}
		// quick path for 0x00
		if b == 0x00 {
			if zerosCnt == 0 {
				zerosStartsAt = unit(i) * 8
			}
			zerosCnt += 8
			continue
		}

		for bitIdx := unit(0); bitIdx < 8; bitIdx++ {
			if b&(1<<bitIdx) != 0 {
				if zerosCnt > 0 {
					s.put(zerosStartsAt, zerosCnt)
					zerosCnt = 0
				}
			} else {
				if zerosCnt == 0 {
					zerosStartsAt = unit(i)*8 + bitIdx
				}
				zerosCnt++
			}
		}
	}
	if zerosCnt > 0 {
		s.put(zerosStartsAt, zerosCnt)
	}
}

// rebuildMaxContinuousFree rebuilds maxContinuousFree. Only when the maximum
// continuous free space is larger than threshold it will convert the state to
// stateValid. Otherwise, it will convert the state to stateExhausted.
func (s *freeSpaces) rebuildMaxContinuousFree(threshold unit) {
	if s.maxContinuousFree.state != stateNeedRebuild {
		panic("unexpected state")
	}

	s.maxContinuousFree.state = stateExhausted
	for i := len(s.buckets) - 1; i >= 0; i-- {
		varLengthB, ok := s.buckets[i].(*varLengthBucket)
		if !ok {
			break
		}
		if len(varLengthB.locations) == 0 {
			continue
		}

		maxLength := threshold
		for _, l := range varLengthB.locations {
			if l.length > maxLength {
				s.maxContinuousFree.bucket = varLengthB
				s.maxContinuousFree.state = stateValid
				s.maxContinuousFree.loc = l
				maxLength = l.length
			}
		}
		break
	}
}

func (s *freeSpaces) put(offset unit, length unit) {
	s.getBucket(length).put(offset, length)
	if s.maxContinuousFree.state == stateExhausted {
		s.maxContinuousFree.state = stateNeedRebuild
	}
}

func (s *freeSpaces) getBucket(length unit) bucket {
	return s.buckets[s.getBucketIdx(length)]
}

func (s *freeSpaces) getBucketIdx(length unit) int {
	if length == 0 {
		panic("length should not be zero")
	}
	if length < oneLengthBucketThreshold {
		return int(length - 1)
	}

	extraOffset := getHighestOneIdx(length) - oneLengthBucketThresholdBits
	return oneLengthBucketThreshold + extraOffset - 1
}

func (s *freeSpaces) take(length unit) (unit, bool) {
	cont := &s.maxContinuousFree
	if cont.state == stateExhausted {
		return 0, false
	}

	// first try to allocate from a free space with exact length

	exactBucket := s.getBucket(length)
	offset, ok := exactBucket.take(length)
	if ok {
		// when it's the same space with maxContinuousFree
		if offset == cont.loc.offset {
			cont.state = stateNeedRebuild
		}
		return offset, true
	}

	// then try to allocate from maxContinuousFree

allocateFromMaxContinuousFree:
	if cont.state == stateNeedRebuild {
		s.rebuildMaxContinuousFree(length)
	}
	if cont.state == stateExhausted {
		return 0, false
	}

	if cont.loc.length < length {
		cont.state = stateNeedRebuild
		goto allocateFromMaxContinuousFree
	}

	oldOffset := cont.loc.offset
	newOffset := oldOffset + length
	newLength := cont.loc.length - length

	if newLength == 0 {
		cont.state = stateNeedRebuild
		cont.bucket.delete(oldOffset)
		return oldOffset, true
	}
	if newLength < oneLengthBucketThreshold {
		cont.state = stateNeedRebuild
		cont.bucket.delete(oldOffset)
		s.put(newOffset, newLength)
		return oldOffset, true
	}
	if newLength < cont.bucket.lengthLowerBound {
		// move the location to a smaller bucket
		newBucket := s.getBucket(newLength).(*varLengthBucket)
		oldIdx := slices.IndexFunc(cont.bucket.locations, func(l *location) bool {
			return l.offset == oldOffset
		})
		l := len(cont.bucket.locations)
		cont.bucket.locations[oldIdx], cont.bucket.locations[l-1] = cont.bucket.locations[l-1], nil
		cont.bucket.locations = cont.bucket.locations[:l-1]
		newBucket.locations = append(newBucket.locations, cont.loc)
		cont.bucket = newBucket
	}

	cont.loc.offset = newOffset
	cont.loc.length = newLength
	return oldOffset, true
}

func (s *freeSpaces) delete(offset, length unit) {
	if offset == s.maxContinuousFree.loc.offset {
		s.maxContinuousFree.state = stateNeedRebuild
	}
	s.getBucket(length).delete(offset)
}

type bucket interface {
	put(offset unit, length unit)
	take(length unit) (unit, bool)
	delete(offset unit)
}

type oneLengthBucket struct {
	length  unit
	offsets []unit
}

func (o *oneLengthBucket) put(offset unit, _ unit) {
	o.offsets = append(o.offsets, offset)
}

func (o *oneLengthBucket) take(length unit) (unit, bool) {
	if length > o.length {
		panic("unexpected length")
	}
	if len(o.offsets) == 0 {
		return 0, false
	}
	offset := o.offsets[len(o.offsets)-1]
	o.offsets = o.offsets[:len(o.offsets)-1]
	return offset, true
}

func (o *oneLengthBucket) delete(offset unit) {
	idx := slices.Index(o.offsets, offset)
	o.offsets[idx] = o.offsets[len(o.offsets)-1]
	o.offsets = o.offsets[:len(o.offsets)-1]
}

type location struct {
	offset unit
	length unit
}

var locationPool = sync.Pool{
	New: func() interface{} {
		return &location{}
	},
}

type varLengthBucket struct {
	lengthLowerBound unit
	locations        []*location
}

func (v *varLengthBucket) put(offset unit, length unit) {
	l := locationPool.Get().(*location)
	l.offset = offset
	l.length = length
	v.locations = append(v.locations, l)
}

func (v *varLengthBucket) take(length unit) (unit, bool) {
	if len(v.locations) == 0 {
		return 0, false
	}

	idx := slices.IndexFunc(v.locations, func(l *location) bool {
		return l.length == length
	})
	if idx == -1 {
		return 0, false
	}
	offset := v.locations[idx].offset
	v.deleteByIdx(idx)
	return offset, true
}

func (v *varLengthBucket) deleteByIdx(idx int) {
	toDelete := v.locations[idx]
	v.locations[idx], v.locations[len(v.locations)-1] = v.locations[len(v.locations)-1], nil
	v.locations = v.locations[:len(v.locations)-1]
	locationPool.Put(toDelete)
}

func (v *varLengthBucket) delete(offset unit) {
	idx := slices.IndexFunc(v.locations, func(l *location) bool {
		return l.offset == offset
	})
	v.deleteByIdx(idx)
}
