# 基本设计

## 元信息设计选择的讨论

考虑到待管理的空间大小为 1TiB、单元粒度为 4KiB，那么单元总数为 256M 个。
单次分配不超过 4MiB，因此单次分配或释放的单元数不超过 1024 个。

初步能想到如下两种数据结构：

1. 位图：使用位图表示每个单元是否被使用，每个单元需要 1bit，总共需要 32MiB 的空间。
2. 记录长度：记录每个单元的“使用状态+连续状态长度”信息，需要花费 1bit+10bit，可以用 2B 存储，总共需要 512MiB 的空间。

在元信息上，需要有如下操作：

- 寻找：寻找连续 N 个未分配的单元，N <= 1024
- 分配：将“寻找”结果标记为已分配
- 释放：将已分配的连续 N 个单元标记为未分配

首先初步检查两种数据结构对于成功的“寻找”操作的性能。假设最坏情况，N = 1024，在 `bench_test.go` 中进行了测试。

```
goos: darwin
goarch: arm64
pkg: github.com/lance6716/disk-management-demo
BenchmarkBitmapCheckContinuousZero
BenchmarkBitmapCheckContinuousZero-8   	186521175	         6.439 ns/op
BenchmarkParseToCheck
BenchmarkParseToCheck-8                	843539036	         1.378 ns/op
```

两种数据结构的速度都足够快。

接下来初步检查两种数据结构对于“分配”和“释放”操作的性能。使用 N = 1024 的写入场景进行模拟，在 `bench_test.go` 中进行测试。

```
goos: darwin
goarch: arm64
pkg: github.com/lance6716/disk-management-demo
BenchmarkBitmapWriteContinuousOne
BenchmarkBitmapWriteContinuousOne-8   	323129762	         3.818 ns/op
BenchmarkEncodeAndWrite
BenchmarkEncodeAndWrite-8             	889015932	         1.338 ns/op
```

同样，两种数据结构的速度都足够快。

在这两种测试里，两种数据结构的性能都足够好，而位图的空间占用更小、几乎不需要额外维护其他特性，因此优先考虑位图。
但是我们还需要考虑失败的“寻找”操作的性能，如果不进行优化，在无法分配 1 个单元的情况下，耗时为 105ms（见 `bench_test.go`）。

```
goos: darwin
goarch: arm64
pkg: github.com/lance6716/disk-management-demo
BenchmarkBitmapCheckFailed128-8   	     724	   1441247 ns/op
BenchmarkBitmapCheckFailed1-8     	      10	 105219108 ns/op
```

这可以通过简单维护一些指向连续未分配单元开头的指针来解决。

这些指针需要携带的信息是起始位置 offset（0~256Mi-1）和长度 length（1~256Mi），分配操作需要实现如下接口：
- put(offset, length)
- takeAtLeast(length) (offset, length)

可以使用桶分治存放这些指针，每个桶的属性是
- lengthLowerBound
- lengthUpperBound
长度在 [lengthLowerBound, lengthUpperBound) 的指针存放在这个桶中。

当桶只能包含一个长度时，我们可以使用一个数组存放指针，在数组尾部实现 put、takeAtLeast。
对于这样的桶，只包含 1 这个长度时，需要存放的元素最多，为 128Mi 个。
需要 28bit 存放 offset，可以使用 4B 来存储，对应的空间占用是 512MiB。

当桶包含多个长度时，takeAtLeast 需要遍历内部的容器，才能找到满足条件的指针。
我们测试桶中至多包含多少个元素时遍历能满足性能要求，发现在桶中包含 1Mi 元素时，最坏情况下耗时约在 0.6ms（见 `bench_test.go`）

```
goos: darwin
goarch: arm64
pkg: github.com/lance6716/disk-management-demo
BenchmarkIter1MiElem
BenchmarkIter1MiElem-8   	    1857	    612238 ns/op
```

如果把 [128, 256) 的长度分配到一个桶中，最坏情况下每次分配的长度都是 128，而已分配和未分配的单元全部相隔，
那么桶中有 256Mi/128/2 = 1Mi 个元素。

因此桶的划分策略是：
- 对于长度小于 128 的指针，在专属的桶中存放
- 对于长度大于等于 128 的指针，存放在长度为 [ floor(log2(length), ceil(log2(length)) ) 的桶中。
共有 127+22 = 149 个桶。

第一类桶使用数组实现，数组中的元素是 offset。
第二类桶使用数组实现，数组中的元素是 (offset, length%lengthLowerBound)。

按照求，删除操作的性能可以低一些，因此我们适当降低对删除操作的要求。
删除操作会引发相邻的未分配空间的合并，因此桶中的指针也需要删除并移动到其他桶中。

对于左侧相邻的未分配空间，可以在位图上找到，时间开销与 `BenchmarkBitmapCheckFailed128` 接近

```
goos: darwin
goarch: arm64
pkg: github.com/lance6716/disk-management-demo
BenchmarkBitmapCheckFailed128-8   	     724	   1441247 ns/op
```

接下来就可以根据左侧和右侧的未分配空间的首地址、长度，从桶中删除它们的指针。
因此接口需要实现
- delete(offset)

第一类桶的元素至多为 128Mi 个，测试删除操作的性能（见 `bench_test.go`）

```
goos: darwin
goarch: arm64
pkg: github.com/lance6716/disk-management-demo
BenchmarkIter128MiElem
BenchmarkIter128MiElem-8   	      12	  85439798 ns/op
```

如果性能不符合要求，可以将桶进一步按照 offset 划分成更小的桶。

## 元信息方案

最终选择的元信息方案是：
- bitmap: [32MiB]byte
- freeSpaces: struct{ getBucket(length) BucketInterface } 

其中 bitmap 被持久化到磁盘，freeSpaces 在初始化时从 bitmap 中构建出来，然后在内存中维护。

### 寻找操作

1. 通过 freeSpaces.getBucket(length).takeAtLeast(length) 获取连续的未分配单元

### 分配操作

1. 使用寻找操作获取连续的未分配单元
2. 将 bitmap 中对应的位标记为已分配
3. 如果该单元还有剩余空间，使用 freeSpaces.getBucket(length-n).put(offset+n, length-n) 将剩余空间放入 freeSpaces 中

### 释放操作

1. 将 bitmap 中对应的位标记为未分配
2. 在 bitmap 上 seek 左右相邻的未分配空间的首地址、长度
3. 从 freeSpaces 中删除这些指针，freeSpaces.getBucket(length).delete(offset)
4. 向 freeSpaces 中添加新的指针，freeSpaces.getBucket(totalLength).put(leftOffset, totalLength)

## 并发调用（下文中实现）

如果单线程的性能可以达到要求，可以将多个线程的请求转发给单线程 worker 完成。
否则可以需要对访问的各个数据结构实现并发接口。

除此之外，并发调用有机会将一个单元分配给多个亚单元级别的分配操作。

# 在基本实现后简单测试性能

## 测试无删除操作的单纯分配

测试单线程每次请求分配 1MiB，见 `impl_manager_test.go`

```
=== RUN   TestAllocDuration
    impl_manager_test.go:216: 1048576 Allocs took 72.166459ms, 68ns/alloc
--- PASS: TestAllocDuration (0.12s)
```

单线程分配的性能足够快，因此实现并发调用时减少争抢开销即可。

# 并发安全设计

首先考虑简单通过锁或者 channel 保护临界区，如果性能足够，可以直接使用这种方式。
测试对于 i++ 的任务，共 1000000 个任务，并发度为 4000 时的耗时变化，见 `impl_manager_test.go`

```
goos: darwin
goarch: arm64
pkg: github.com/lance6716/disk-management-demo
BenchmarkCallSerialized-8       	     572	   2084792 ns/op
BenchmarkCallChanUnbuffered-8   	       3	 336400222 ns/op
BenchmarkCallChanBuffered-8     	       3	 381394472 ns/op
BenchmarkCallAcquireLock-8      	       8	 140232932 ns/op
PASS
```

使用锁的方式性能最好，每个任务耗时从 2ns 增加到 140ns。
虽然相较于真正的 Alloc 耗时 68ns 很慢，但期望这种耗时也能够容忍。

