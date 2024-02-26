package disk_management_demo

import "slices"

const (
	oneLengthBucketThresholdBits = 7
	oneLengthBucketThreshold     = 1 << oneLengthBucketThresholdBits // 128
	varLengthBucketCnt           = unitTotalCntBits - oneLengthBucketThresholdBits + 1
	totalBucketCnt               = varLengthBucketCnt + oneLengthBucketThreshold - 1
)

type freeSpacesTp struct {
	buckets [totalBucketCnt]bucket
}

func newFreeSpaces() *freeSpacesTp {
	s := &freeSpacesTp{}
	for i := range s.buckets {
		if i+1 < oneLengthBucketThreshold {
			// buckets[0] has length 1, ... buckets[126] has length 127
			s.buckets[i] = &oneLengthBucket{length: uint8(i + 1)}
		} else {
			extraExponent := i + 1 - oneLengthBucketThreshold
			s.buckets[i] = &varLengthBucket{
				lengthLowerBound: oneLengthBucketThreshold * (1 << extraExponent),
			}
		}
	}
	return s
}

func (s *freeSpacesTp) put(offset uint32, length uint32) {
	s.getBucket(length).put(offset, length)
}

// only used in tests
func (s *freeSpacesTp) getBucket(length uint32) bucket {
	return s.buckets[s.getBucketIdx(length)]
}

func (s *freeSpacesTp) getBucketIdx(length uint32) int {
	if length == 0 {
		panic("length should not be zero")
	}
	if length < oneLengthBucketThreshold {
		return int(length - 1)
	}

	extraOffset := getHighestOneIdx(length) - oneLengthBucketThresholdBits
	return oneLengthBucketThreshold + extraOffset - 1
}

func (s *freeSpacesTp) takeAtLeast(length uint32) (uint32, uint32, bool) {
	for i := s.getBucketIdx(length); i < len(s.buckets); i++ {
		offset, l, ok := s.buckets[i].takeAtLeast(length)
		if ok {
			return offset, l, true
		}
	}
	return 0, 0, false
}

func (s *freeSpacesTp) delete(offset, length uint32) {
	s.getBucket(length).delete(offset)
}

type bucket interface {
	put(offset uint32, length uint32)
	takeAtLeast(length uint32) (uint32, uint32, bool)
	delete(offset uint32)
}

type oneLengthBucket struct {
	length  uint8
	offsets []uint32
}

func (o *oneLengthBucket) put(offset uint32, _ uint32) {
	o.offsets = append(o.offsets, offset)
}

func (o *oneLengthBucket) takeAtLeast(length uint32) (uint32, uint32, bool) {
	if length > uint32(o.length) {
		panic("unexpected length")
	}
	if len(o.offsets) == 0 {
		return 0, 0, false
	}
	offset := o.offsets[len(o.offsets)-1]
	o.offsets = o.offsets[:len(o.offsets)-1]
	return offset, uint32(o.length), true
}

func (o *oneLengthBucket) delete(offset uint32) {
	idx := slices.Index(o.offsets, offset)
	o.offsets[idx] = o.offsets[len(o.offsets)-1]
	o.offsets = o.offsets[:len(o.offsets)-1]
}

type location struct {
	offset uint32
	length uint32
}

type varLengthBucket struct {
	lengthLowerBound uint32
	locations        []location
}

func (v *varLengthBucket) put(offset uint32, length uint32) {
	v.locations = append(v.locations, location{offset: offset, length: length})
}

func (v *varLengthBucket) takeAtLeast(length uint32) (uint32, uint32, bool) {
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

func (v *varLengthBucket) delete(offset uint32) {
	idx := slices.IndexFunc(v.locations, func(l location) bool {
		return l.offset == offset
	})
	v.locations[idx] = v.locations[len(v.locations)-1]
	v.locations = v.locations[:len(v.locations)-1]
}
