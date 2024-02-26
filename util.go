package disk_management_demo

func getHighestOneIdx(n uint32) int {
	idx := -1
	for n > 0 {
		n >>= 1
		idx++
	}
	return idx
}

func bytesToUnitCnt(bytes int64) uint32 {
	return uint32((bytes + unitSize - 1) / unitSize)
}

func unitOffsetToBytes(offset uint32) int64 {
	return int64(offset) * unitSize
}

func allocInBitmap(bitmap []byte, offset, length uint32) {
	zeroBitsBeforeFullByte := offset % 8
	if zeroBitsBeforeFullByte != 0 {
		b := byte(0xFF << zeroBitsBeforeFullByte)
		oneBitsBeforeFullByte := 8 - zeroBitsBeforeFullByte
		if length < oneBitsBeforeFullByte {
			b &= 0xFF >> (oneBitsBeforeFullByte - length)
			length = 0
		} else {
			length -= oneBitsBeforeFullByte
		}

		byteIdx := offset / 8
		bitmap[byteIdx] |= b

		offset += oneBitsBeforeFullByte
	}

	// TODO(lance6716): check performance if use copy(dst, ones) instead of loop
	for length >= 8 {
		bitmap[offset/8] = 0xFF
		offset += 8
		length -= 8
	}

	if length == 0 {
		return
	}
	zeroBitsInLastUnit := 8 - length
	b := byte(0xFF >> zeroBitsInLastUnit)
	bitmap[offset/8] |= b
}

func freeInBitmap(bitmap []byte, offset, length uint32) {
	oneBitsBeforeFullByte := offset % 8
	if oneBitsBeforeFullByte != 0 {
		zeroBitsBeforeFullByte := 8 - oneBitsBeforeFullByte
		b := byte(0xFF >> zeroBitsBeforeFullByte)
		if length < zeroBitsBeforeFullByte {
			b |= 0xFF << (8 - (zeroBitsBeforeFullByte - length))
			length = 0
		} else {
			length -= zeroBitsBeforeFullByte
		}

		byteIdx := offset / 8
		bitmap[byteIdx] &= b

		offset += zeroBitsBeforeFullByte
	}

	for length >= 8 {
		bitmap[offset/8] = 0x00
		offset += 8
		length -= 8
	}

	if length == 0 {
		return
	}
	zeroBitsInLastUnit := length
	b := byte(0xFF << zeroBitsInLastUnit)
	bitmap[offset/8] &= b
}

func findLeadingZerosCnt(bitmap []byte, startOffset uint32) uint32 {
	if startOffset == unitTotalCnt {
		return 0
	}
	if startOffset > unitTotalCnt {
		panic("unexpected startOffset")
	}

	byteIdx := startOffset / 8
	bitIdx := startOffset % 8

	ret := uint32(0)
	b := bitmap[byteIdx]
	for ; bitIdx < 8; bitIdx++ {
		if b&(1<<bitIdx) != 0 {
			return ret
		}
		ret++
	}

	for byteIdx++; byteIdx < metadataSize; byteIdx++ {
		if bitmap[byteIdx] != 0 {
			break
		}
		ret += 8
	}
	if byteIdx == metadataSize {
		return ret
	}
	bitIdx = 0
	b = bitmap[byteIdx]
	for ; bitIdx < 8; bitIdx++ {
		if b&(1<<bitIdx) != 0 {
			return ret
		}
		ret++
	}

	return ret
}

func findTrailingZerosCnt(bitmap []byte, endOffset uint32) uint32 {
	if endOffset == 0 {
		return 0
	}
	if endOffset > unitTotalCnt {
		panic("unexpected endOffset")
	}

	byteIdx := int((endOffset - 1) / 8)
	bitIdx := int((endOffset - 1) % 8)

	ret := uint32(0)
	b := bitmap[byteIdx]
	for ; bitIdx >= 0; bitIdx-- {
		if b&(1<<bitIdx) != 0 {
			return ret
		}
		ret++
	}

	for byteIdx--; byteIdx >= 0; byteIdx-- {
		if bitmap[byteIdx] != 0 {
			break
		}
		ret += 8
	}
	if byteIdx == -1 {
		return ret
	}
	bitIdx = 7
	b = bitmap[byteIdx]
	for ; bitIdx >= 0; bitIdx-- {
		if b&(1<<bitIdx) != 0 {
			return ret
		}
		ret++
	}

	return ret
}
