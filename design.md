# 设计 1

## 元信息设计

考虑到待管理的空间大小为 1TiB、单元粒度为 4KiB，那么单元总数为 256M 个。
设计 1 想从“连续分配或释放”入手管理元信息，那么有如下信息需要记录：

- 首地址，取值有 256M 个，28bit
- 分配标志，1bit
- 长度，取值有 256M 个，28bit

设计 1 以比较直观的方式记录元信息。
为了方便以首地址进行寻址，我们为 256M 个首地址分配定长的数据结构，这样的数据结构需要 29 bit，可以用 4B 存储。
因此占用空间为 256M * 4B = 1GiB。

如果将 1GiB 的空间始终加载在内存中，有两个缺点：
首先是 1GiB 的内存占用在主观感受上有些大；
其次是元信息格式有很大的浪费，在连续分配 N 个单元时，除首个单元的 N-1 个单元的元信息都是没有使用的，白白占据了内存。
因此决定将这 1GiB 存储在磁盘上。

对于分配标志，规定 1 为已分配，0 为未分配。

除此之外，还需要记录一些额外的信息，例如整个空间是否已经格式化。
设计 1 在空间的最头部留出 4B 的空间维护这些信息。

## 存储布局

待管理空间为

```
[0B ~ 3B ][4B ~ 7B]...[1GiB ~ (1Gi+3)B]
| 额外信息 | 单元0  |...| 单元(256M-1)   |
```

单元

```
[0bit ~ 27bit][28bit    ][29bit ~ 31bit]
| 长度        | 分配标志  | 保留          |
```

## 元信息更新

首先不考虑并发调用的场景。
对于每次分配和释放的调用，多个单元需要更新，我们假设已经实现了原子提交功能 `Commit([]UnitModify)`。

接下来我们需要计算需要更新的若干单元，对于分配操作 `Alloc(n int)`，大体流程如下

```go
var freeUnitIdx int32

func Alloc(n int) (SpaceHandle, error) {
	freeUnits := decodeUnits(freeUnitIdx)
	switch {
	case n < freeUnits.length:
		modifies := []UnitModify{
		    {freeUnitIdx, StateUsed, n},
            {freeUnitIdx + n, StateFree, freeUnits.length - n},
        }
		Commit(modifies)
		oldFreeUnitIdx := freeUnitIdx
		freeUnitIdx += n
		return NewSpaceHandle(oldFreeUnitIdx, n), nil
	case n == freeUnits.length:
		modifies := []UnitModify{
		    {freeUnitIdx, StateUsed, n},
        }
		modifies, nextFreeUnitIdx = mergeFollowingUnits(modifies)
        Commit(modifies)
		oldFreeUnitIdx := freeUnitIdx
		freeUnitIdx = nextFreeUnitIdx
		return NewSpaceHandle(oldFreeUnitIdx, n), nil
	case n > freeUnits.length:
		unitIdx := findEnoughSpace(n)
		if unitIdx < 0 {
			return nil, ErrNoEnoughSpace
		}

        freeUnitIdx = unitIdx
		return Alloc(n)
    }
}
```

对于释放操作 `Free(handle SpaceHandle, n int)`，大体流程如下

```go
func Free(handle SpaceHandle, n int) error {
    modifies := []UnitModify{
        {handle.ID, StateFree, n},
    }
    modifies, nextFreeUnitIdx = mergeFollowingUnits(modifies)
    Commit(modifies)
    freeUnitIdx = nextFreeUnitIdx
    return nil
}
```

预期如上的操作，在磁盘上 seek、read 的耗时占比较大，可以考虑 `freeUnitIdx` 将后续的若干单元预加载到内存中。

另一个提升性能的角度是，注意到上面的操作几乎是局部的（除了 `findEnoughSpace` 以及 `mergeFollowingUnits`），
因此在合适的加锁或者分区场景下，可以考虑支持并发更新。
