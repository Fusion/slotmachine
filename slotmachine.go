package slotmachine

import (
	"fmt"
	"golang.org/x/exp/constraints"
	"math"
	"sync"
)

type Validated uint8

const (
	InBound Validated = iota
	OutOfBound
)

type Boundaries struct {
	Lower int
	Upper int
}

type SlotMachine[T constraints.Integer, V any] struct {
	slice        *[]V
	empty        V
	boundaries   Boundaries
	bucketSize   uint8 // A bucket can only be as wide as an integer type's number of bits...
	full         T
	bucketLevels *[][]T
	m            sync.Mutex
	debug        bool
}

func New[T constraints.Integer, V any](
	slice *[]V,
	empty V,
	bucketSize uint8,
	boundaries *Boundaries,
) (*SlotMachine[T, V], error) {

	if math.Ceil(math.Log2(float64(bucketSize))) != math.Floor(math.Log2(float64(bucketSize))) {
		return nil, fmt.Errorf("bucket size must be a power of 2")
	}
	width := len(*slice)
	if math.Ceil(math.Log2(float64(width))) != math.Floor(math.Log2(float64(width))) {
		return nil, fmt.Errorf("for performance, the slice's size needs to be 2-aligned; suggest you resize to %d and set upper bound",
			int(math.Pow(2.0, math.Ceil(math.Log2(float64(len(*slice)))))))
	}

	var bucketLevels [][]T
	for {
		bucketCount := width / int(bucketSize)
		if bucketCount == 0 {
			bucketCount = 1
		}
		buckets := make([]T, bucketCount)
		bucketLevels = append([][]T{buckets}, bucketLevels...)
		if bucketCount == 1 {
			break
		}
		width = bucketCount
	}

	var bdrs Boundaries
	if boundaries != nil {
		bdrs = *boundaries
	} else {
		bdrs = Boundaries{0, len(*slice) - 1}
	}

	bucketFull := (1 << bucketSize) - 1
	return &SlotMachine[T, V]{
		slice:        slice,
		empty:        empty,
		bucketSize:   bucketSize,
		full:         T(bucketFull),
		bucketLevels: &bucketLevels,
		boundaries:   bdrs,
	}, nil
}

func (s *SlotMachine[T, V]) DumpLayout() {
	width := len(*s.slice)
	fmt.Printf("Slice size: %d (Usable slots: %d - %d)\n", width, (*s).boundaries.Lower, (*s).boundaries.Upper)
	fmt.Printf("Bucket size: %d\n", (*s).bucketSize)
	for {
		bucketCount := width / int(s.bucketSize)
		if bucketCount == 0 {
			bucketCount = 1
		}
		fmt.Printf("Buckets per level: %d\n", bucketCount)
		if bucketCount == 1 {
			break
		}
		width = bucketCount
	}
}

func (s *SlotMachine[T, V]) checkBoundaries(slotidx T) Validated {
	if slotidx < T(s.boundaries.Lower) || slotidx > T(s.boundaries.Upper) {
		return OutOfBound
	}
	return InBound
}

func (s *SlotMachine[T, V]) Set(slotidx T, value V) error {
	if s.checkBoundaries(slotidx) == OutOfBound {
		return fmt.Errorf("slot index %d is out of bounds", slotidx)
	}

	(*s.slice)[slotidx] = value

	level := (*s.bucketLevels)[len(*s.bucketLevels)-1]
	slicesize := len(*s.slice) / len(level)
	bucket, offset := int(slotidx)/slicesize, int(slotidx)%slicesize
	level[bucket] |= (1 << offset)

	if level[bucket] != (*s).full {
		return nil
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

	return nil
}

func (s *SlotMachine[T, V]) Unset(slotidx T) error {
	if s.checkBoundaries(slotidx) == OutOfBound {
		return fmt.Errorf("slot index %d is out of bounds", slotidx)
	}

	emptyVal := (*s).empty
	var emptyIf any = emptyVal
	(*s.slice)[slotidx] = emptyIf.(V)

	level := (*s.bucketLevels)[len(*s.bucketLevels)-1]
	slicesize := len(*s.slice) / len(level)
	bucket, offset := int(slotidx)/slicesize, int(slotidx)%slicesize
	level[bucket] &^= (1 << offset)

	if level[bucket] == (*s).full {
		return nil
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

	return nil
}

func (s *SlotMachine[T, V]) SyncSet(slotidx T, value V) error {
	(*s).m.Lock()
	defer (*s).m.Unlock()

	return s.Set(slotidx, value)
}

func (s *SlotMachine[T, V]) SyncUnset(slotidx T) error {
	(*s).m.Lock()
	defer (*s).m.Unlock()

	return s.Unset(slotidx)
}

func (s *SlotMachine[T, V]) SyncBookAndSet(value V) (T, error) {
	var level []T
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
			if s.Set(T(slot), value) != nil {
				return 0, fmt.Errorf("SlotMachine: No usable slot")
			}
			return T(slot), nil
		}
	}
	return 0, fmt.Errorf("SlotMachine: No usable slot")
}
