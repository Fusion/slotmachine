package slotmachine

import (
	"fmt"
	"golang.org/x/exp/constraints"
	"math"
	"sync"
)

type ConcurrencyModel uint8

const (
	NoConcurrency ConcurrencyModel = iota
	SyncConcurrency
	ChannelConcurrency
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

type SlotMachineStruct[T constraints.Integer, V any] struct {
	slice        *[]V
	empty        V
	boundaries   Boundaries
	bucketSize   uint8 // A bucket can only be as wide as an integer type's number of bits...
	full         T
	bucketLevels *[][]T
	m            sync.Mutex
	debug        bool
	available    T
}

type SlotMachine[T constraints.Integer, V any] interface {
	Init(
		slice *[]V,
		empty V,
		bucketSize uint8,
		full T,
		bucketLevels *[][]T,
		boundaries *Boundaries,
	)
	Set(slotidx T, value V) (T, error)
	Unset(slotidx T) (T, error)
	BookAndSet(value V) (T, T, error)
	BookAndSetBatch(slotcount T, value V) ([]T, T, error)
	DumpLayout()
}

func (s *SlotMachineStruct[T, V]) DumpLayout() {
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

func (s *SlotMachineStruct[T, V]) checkBoundaries(slotidx T) Validated {
	if slotidx < T(s.boundaries.Lower) || slotidx > T(s.boundaries.Upper) {
		return OutOfBound
	}
	return InBound
}

func (s *SlotMachineStruct[T, V]) set(slotidx T, value V) (T, error) {
	if s.checkBoundaries(slotidx) == OutOfBound {
		return s.available, fmt.Errorf("slot index %d is out of bounds", slotidx)
	}

	(*s.slice)[slotidx] = value

	level := (*s.bucketLevels)[len(*s.bucketLevels)-1]
	slicesize := len(*s.slice) / len(level)
	bucket, offset := int(slotidx)/slicesize, int(slotidx)%slicesize
	level[bucket] |= (1 << offset)

	s.available--

	if level[bucket] != (*s).full {
		return s.available, nil
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

	return s.available, nil
}

func (s *SlotMachineStruct[T, V]) unset(slotidx T) (T, error) {
	if s.checkBoundaries(slotidx) == OutOfBound {
		return s.available, fmt.Errorf("slot index %d is out of bounds", slotidx)
	}

	emptyVal := (*s).empty
	var emptyIf any = emptyVal
	(*s.slice)[slotidx] = emptyIf.(V)

	level := (*s.bucketLevels)[len(*s.bucketLevels)-1]
	slicesize := len(*s.slice) / len(level)
	bucket, offset := int(slotidx)/slicesize, int(slotidx)%slicesize
	level[bucket] &^= (1 << offset)

	s.available++

	if level[bucket] == (*s).full {
		return s.available, nil
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

	return s.available, nil
}

func (s *SlotMachineStruct[T, V]) bookAndSet(value V) (T, T, error) {
	var level []T
	var found bool
	var bucket int

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
		return 0, s.available, fmt.Errorf("SlotMachine: No available slot")
	}
	slicesize := len(*s.slice) / len(level)
	position := bucket * slicesize
	if (*s).debug {
		fmt.Printf("Found bucket %d position=%d slotidx=%d\n", bucket, bucket*slicesize, level[bucket])
	}
	for i := 0; i < +slicesize; i++ {
		if level[bucket]&(1<<i) == 0 {
			slot := position + i
			_, err := s.set(T(slot), value)
			if err != nil {
				return 0, s.available, fmt.Errorf("SlotMachine: No usable slot: %s", err)
			}
			return T(slot), s.available, nil
		}
	}
	return 0, s.available, fmt.Errorf("SlotMachine: No usable slot")
}

func New[T constraints.Integer, V any](
	cmodel ConcurrencyModel,
	slice *[]V,
	empty V,
	bucketSize uint8,
	boundaries *Boundaries,
) (SlotMachine[T, V], error) {

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

	var bdrs *Boundaries
	if boundaries != nil {
		bdrs = boundaries
	} else {
		bdrs = &Boundaries{0, len(*slice) - 1}
	}

	bucketFull := (1 << bucketSize) - 1

	switch cmodel {
	case NoConcurrency:
		sm := NoConcurrencySlotMachine[T, V]{}
		sm.Init(
			slice,
			empty,
			bucketSize,
			T(bucketFull),
			&bucketLevels,
			bdrs,
		)
		return &sm, nil
	case SyncConcurrency:
		sm := SyncConcurrencySlotMachine[T, V]{}
		sm.Init(
			slice,
			empty,
			bucketSize,
			T(bucketFull),
			&bucketLevels,
			bdrs,
		)
		return &sm, nil
	case ChannelConcurrency:
		sm := ChannelConcurrencySlotMachine[T, V]{}
		sm.Init(
			slice,
			empty,
			bucketSize,
			T(bucketFull),
			&bucketLevels,
			bdrs,
		)
		return &sm, nil
	default:
		return nil, fmt.Errorf("Unknown concurrency model")
	}
}

type NoConcurrencySlotMachine[T constraints.Integer, V any] struct {
	st SlotMachineStruct[T, V]
}

func (s *NoConcurrencySlotMachine[T, V]) Init(
	slice *[]V,
	empty V,
	bucketSize uint8,
	full T,
	bucketLevels *[][]T,
	boundaries *Boundaries,
) {
	s.st.slice = slice
	s.st.empty = empty
	s.st.bucketSize = bucketSize
	s.st.full = T(full)
	s.st.bucketLevels = bucketLevels
	s.st.boundaries = *boundaries
	s.st.available = T(s.st.boundaries.Upper) - T(s.st.boundaries.Lower)
}

func (s *NoConcurrencySlotMachine[T, V]) Set(slotidx T, value V) (T, error) {
	return s.st.set(slotidx, value)
}

func (s *NoConcurrencySlotMachine[T, V]) Unset(slotidx T) (T, error) {
	return s.st.unset(slotidx)
}

func (s *NoConcurrencySlotMachine[T, V]) BookAndSet(value V) (T, T, error) {
	return s.st.bookAndSet(value)
}

func (s *NoConcurrencySlotMachine[T, V]) BookAndSetBatch(slotcount T, value V) ([]T, T, error) {
	slots := []T{}
	var n T
	var err error
	var available T
	for i := 0; i < int(slotcount); i++ {
		n, available, err = s.st.bookAndSet(value)
		if err != nil {
			return nil, available, err
		}
		slots = append(slots, n)
	}

	return slots, available, nil
}

func (s *NoConcurrencySlotMachine[T, V]) DumpLayout() {
	s.st.DumpLayout()
}

type SyncConcurrencySlotMachine[T constraints.Integer, V any] struct {
	st SlotMachineStruct[T, V]
}

func (s *SyncConcurrencySlotMachine[T, V]) Init(
	slice *[]V,
	empty V,
	bucketSize uint8,
	full T,
	bucketLevels *[][]T,
	boundaries *Boundaries,
) {
	s.st.slice = slice
	s.st.empty = empty
	s.st.bucketSize = bucketSize
	s.st.full = T(full)
	s.st.bucketLevels = bucketLevels
	s.st.boundaries = *boundaries
	s.st.available = T(s.st.boundaries.Upper) - T(s.st.boundaries.Lower)
}

func (s *SyncConcurrencySlotMachine[T, V]) Set(slotidx T, value V) (T, error) {
	s.st.m.Lock()
	defer s.st.m.Unlock()

	return s.st.set(slotidx, value)
}

func (s *SyncConcurrencySlotMachine[T, V]) Unset(slotidx T) (T, error) {
	s.st.m.Lock()
	defer s.st.m.Unlock()

	return s.st.unset(slotidx)
}

func (s *SyncConcurrencySlotMachine[T, V]) BookAndSet(value V) (T, T, error) {
	s.st.m.Lock()
	defer s.st.m.Unlock()

	return s.st.bookAndSet(value)
}

func (s *SyncConcurrencySlotMachine[T, V]) BookAndSetBatch(slotcount T, value V) ([]T, T, error) {
	s.st.m.Lock()
	defer s.st.m.Unlock()

	slots := []T{}
	var n T
	var available T
	var err error
	for i := 0; i < int(slotcount); i++ {
		n, available, err = s.st.bookAndSet(value)
		if err != nil {
			return nil, available, err
		}
		slots = append(slots, n)
	}

	return slots, available, nil
}

func (s *SyncConcurrencySlotMachine[T, V]) DumpLayout() {
	s.st.DumpLayout()
}

type TransactionType uint8

const (
	TransactionSet TransactionType = iota
	TransactionUnset
	TransactionBookAndSet
)

type response[T constraints.Integer] struct {
	slotidx   *T
	available T
	err       *error
}

type transact[T constraints.Integer, V any] struct {
	ttype    TransactionType
	slotidx  T
	value    V
	response chan response[T]
}

type ChannelConcurrencySlotMachine[T constraints.Integer, V any] struct {
	st         SlotMachineStruct[T, V]
	transactor chan *transact[T, V]
}

func (s *ChannelConcurrencySlotMachine[T, V]) Init(
	slice *[]V,
	empty V,
	bucketSize uint8,
	full T,
	bucketLevels *[][]T,
	boundaries *Boundaries,
) {
	s.st.slice = slice
	s.st.empty = empty
	s.st.bucketSize = bucketSize
	s.st.full = T(full)
	s.st.bucketLevels = bucketLevels
	s.st.boundaries = *boundaries
	s.st.available = T(s.st.boundaries.Upper) - T(s.st.boundaries.Lower)

	s.transactor = make(chan *transact[T, V], 8)
	go func() {
		for {
			select {
			case transaction := <-s.transactor:
				switch transaction.ttype {
				case TransactionSet:
					available, err := s.st.set(transaction.slotidx, transaction.value)
					transaction.response <- response[T]{nil, available, &err}
				case TransactionUnset:
					available, err := s.st.unset(transaction.slotidx)
					transaction.response <- response[T]{nil, available, &err}
				case TransactionBookAndSet:
					n, available, err := s.st.bookAndSet(transaction.value)
					transaction.response <- response[T]{&n, available, &err}
				}
			}
		}
	}()
}

func (s *ChannelConcurrencySlotMachine[T, V]) Set(slotidx T, value V) (T, error) {
	tr := &transact[T, V]{ttype: TransactionSet, slotidx: slotidx, value: value, response: make(chan response[T])}
	s.transactor <- tr
	response := <-tr.response
	return response.available, *response.err
}

func (s *ChannelConcurrencySlotMachine[T, V]) Unset(slotidx T) (T, error) {
	tr := &transact[T, V]{ttype: TransactionUnset, slotidx: slotidx, response: make(chan response[T])}
	s.transactor <- tr
	response := <-tr.response
	return response.available, *response.err
}

func (s *ChannelConcurrencySlotMachine[T, V]) BookAndSet(value V) (T, T, error) {
	tr := &transact[T, V]{ttype: TransactionBookAndSet, value: value, response: make(chan response[T])}
	s.transactor <- tr
	response := <-tr.response
	return *response.slotidx, response.available, *response.err
}

// Warning! At this time, this  function does not return consecutive slots -- it should when using Sync
func (s *ChannelConcurrencySlotMachine[T, V]) BookAndSetBatch(slotcount T, value V) ([]T, T, error) {
	slots := []T{}
	var n T
	var available T
	var err error
	for i := 0; i < int(slotcount); i++ {
		tr := &transact[T, V]{ttype: TransactionBookAndSet, value: value, response: make(chan response[T])}
		s.transactor <- tr
		response := <-tr.response
		n, available, err = *response.slotidx, response.available, *response.err
		if err != nil {
			return nil, available, err
		}
		slots = append(slots, n)
	}

	return slots, available, nil
}

func (s *ChannelConcurrencySlotMachine[T, V]) DumpLayout() {
	s.st.DumpLayout()
}
