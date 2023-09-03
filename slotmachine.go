package slotmachine

import (
	"fmt"
	"golang.org/x/exp/constraints"
	"sync"
)

type SlotMachine[T constraints.Integer, V any] struct {
	slice        *[]V
	empty        V
	bucketSize   uint8
	full         uint16
	bucketLevels *[][]uint16
	m            sync.Mutex
	debug        bool
}

func New[T constraints.Integer, V any](
	slice *[]V,
	empty V,
	bucketSize uint8,
) *SlotMachine[T, V] {

	var bucketLevels [][]uint16
	width := len(*slice)
	for {
		bucketCount := width / int(bucketSize)
		buckets := make([]uint16, bucketCount)
		bucketLevels = append([][]uint16{buckets}, bucketLevels...)
		if bucketCount == 1 {
			break
		}
		width = bucketCount
	}

	bucketFull := (1 << bucketSize) - 1
	return &SlotMachine[T, V]{
		slice:        slice,
		empty:        empty,
		bucketSize:   bucketSize,
		full:         uint16(bucketFull),
		bucketLevels: &bucketLevels,
	}
}

func (s *SlotMachine[T, V]) Set(slotidx T, value V) {
	(*s.slice)[slotidx] = value

	level := (*s.bucketLevels)[len(*s.bucketLevels)-1]
	slicesize := len(*s.slice) / len(level)
	bucket, offset := int(slotidx)/slicesize, int(slotidx)%slicesize
	level[bucket] |= (1 << offset)

	if level[bucket] != (*s).full {
		return
	}
	if (*s).debug {
		fmt.Printf("bucketfull, (full=%d) slotidx=%d -> bucket=%d (width=%d), bounds=%d-%d\n", level[bucket], slotidx, bucket, len(level), bucket*slicesize, bucket*slicesize+slicesize-1)
	}
	for levelidx := len(*s.bucketLevels) - 2; levelidx >= 0; levelidx-- {
		level = (*s.bucketLevels)[levelidx]
		slicesize = len(*s.slice) / len(level)
		bucket = int(slotidx) / slicesize
		offset = (int(slotidx) % slicesize) / (slicesize / int((*s).bucketSize))
		level[bucket] |= (1 << offset)
		if level[bucket] != (*s).full {
			break
		}
		if (*s).debug {
			fmt.Printf("parent bucket is %d, slicesize=%d, bounds=%d-%d offset=%d newv=%d\n", bucket, slicesize, bucket*slicesize, bucket*slicesize+slicesize-1, offset, level[bucket])
		}
	}
}

func (s *SlotMachine[T, V]) Unset(slotidx T) {
	emptyVal := (*s).empty
	var emptyIf any = emptyVal
	(*s.slice)[slotidx] = emptyIf.(V)

	level := (*s.bucketLevels)[len(*s.bucketLevels)-1]
	slicesize := len(*s.slice) / len(level)
	bucket, offset := int(slotidx)/slicesize, int(slotidx)%slicesize
	level[bucket] &^= (1 << offset)

	if level[bucket] == (*s).full {
		return
	}
	for levelidx := len(*s.bucketLevels) - 2; levelidx >= 0; levelidx-- {
		level = (*s.bucketLevels)[levelidx]
		slicesize = len(*s.slice) / len(level)
		bucket = int(slotidx) / slicesize
		offset = (int(slotidx) % slicesize) / (slicesize / int((*s).bucketSize))
		level[bucket] &^= (1 << offset)
		if level[bucket] == (*s).full {
			break
		}
	}
}

func (s *SlotMachine[T, V]) SyncSet(slotidx T, value V) {
	(*s).m.Lock()
	defer (*s).m.Unlock()

	s.Set(slotidx, value)
}

func (s *SlotMachine[T, V]) SyncUnset(slotidx T) {
	(*s).m.Lock()
	defer (*s).m.Unlock()

	s.Unset(slotidx)
}

func (s *SlotMachine[T, V]) SyncBookAndSet(value V) (T, error) {
	var level []uint16
	var found bool
	var bucket int

	(*s).m.Lock()
	defer (*s).m.Unlock()

	for levelidx := 0; levelidx < len(*s.bucketLevels); levelidx++ {
		found = false
		level = (*s.bucketLevels)[levelidx]
		for bucket = 0; bucket < len(level); bucket++ {
			if level[bucket] != (*s).full {
				if (*s).debug {
					fmt.Printf("Level %d, Found slice %d\n", levelidx, bucket)
				}
				found = true
				break
			} else {
				slicesize := len(*s.slice) / len(level)
				if (*s).debug {
					fmt.Printf("Level %d, slice %d 9[[%d-%d] is full (%d)\n", levelidx, bucket, bucket*slicesize, bucket*slicesize+slicesize-1, level[bucket])
				}
			}
		}
	}
	if !found {
		return 0, fmt.Errorf("SlotMachine: No available slot")
	}
	slicesize := len(*s.slice) / len(level)
	position := bucket * slicesize
	if (*s).debug {
		fmt.Printf("Found bucket %d position=%d slotidx=%d\n", bucket, bucket*slicesize, level[bucket])
	}
	for i := 0; i < +slicesize; i++ {
		if level[bucket]&(1<<i) == 0 {
			slot := position + i
			s.Set(T(slot), value)
			return T(slot), nil
		}
	}
	return 0, fmt.Errorf("SlotMachine: No usable slot")
}
