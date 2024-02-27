package disk_management_demo

import "slices"

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
}

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

func (s *freeSpaces) put(offset unit, length unit) {
	s.getBucket(length).put(offset, length)
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

func (s *freeSpaces) takeAtLeast(length unit) (unit, unit, bool) {
	for i := s.getBucketIdx(length); i < len(s.buckets); i++ {
		offset, l, ok := s.buckets[i].takeAtLeast(length)
		if ok {
			return offset, l, true
		}
	}
	return 0, 0, false
}

func (s *freeSpaces) delete(offset, length unit) {
	s.getBucket(length).delete(offset)
}

type bucket interface {
	put(offset unit, length unit)
	takeAtLeast(length unit) (unit, unit, bool)
	delete(offset unit)
}

type oneLengthBucket struct {
	length  unit
	offsets []unit
}

func (o *oneLengthBucket) put(offset unit, _ unit) {
	o.offsets = append(o.offsets, offset)
}

func (o *oneLengthBucket) takeAtLeast(length unit) (unit, unit, bool) {
	if length > o.length {
		panic("unexpected length")
	}
	if len(o.offsets) == 0 {
		return 0, 0, false
	}
	offset := o.offsets[len(o.offsets)-1]
	o.offsets = o.offsets[:len(o.offsets)-1]
	return offset, o.length, true
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

type varLengthBucket struct {
	lengthLowerBound unit
	locations        []location
}

func (v *varLengthBucket) put(offset unit, length unit) {
	v.locations = append(v.locations, location{offset: offset, length: length})
}

func (v *varLengthBucket) takeAtLeast(length unit) (unit, unit, bool) {
	if len(v.locations) == 0 {
		return 0, 0, false
	}

	gotIdx := -1
	for i, l := range v.locations {
		if l.length >= length {
			gotIdx = i
			break
		}
	}
	if gotIdx == -1 {
		return 0, 0, false
	}

	l := v.locations[gotIdx]
	v.locations[gotIdx] = v.locations[len(v.locations)-1]
	v.locations = v.locations[:len(v.locations)-1]
	return l.offset, l.length, true
}

func (v *varLengthBucket) delete(offset unit) {
	idx := slices.IndexFunc(v.locations, func(l location) bool {
		return l.offset == offset
	})
	v.locations[idx] = v.locations[len(v.locations)-1]
	v.locations = v.locations[:len(v.locations)-1]
}
