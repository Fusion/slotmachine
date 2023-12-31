package slotmachine

import (
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/exp/constraints"
)

type SMTester[T constraints.Integer, V constraints.Integer] struct {
}

func (s *SMTester[T, V]) Exercise(t *testing.T, workSlice *[]V, sliceSize int, bucketSize int, boundaries *Boundaries) {
	var compareto []int
	if boundaries == nil {
		compareto = []int{32751, 32750, 4767, 4767, 31768, 31763}
	} else {
		compareto = []int{49984, 49983, 22000, 22000, 49001, 48996}
	}
	var added T
	var available uint
	sm, err := New[T, V](
		SyncConcurrency,
		workSlice,
		0,
		uint8(bucketSize),
		boundaries)
	if err != nil {
		if bucketSize != 14 && len(*workSlice) != 60000 {
			t.Errorf("failed to create slot machine: %s", err)
		}
		return
	}
	if bucketSize == 14 {
		t.Error("a non-power of 2 bucket size should  have failed")
		return
	}
	if len(*workSlice) == 60000 {
		t.Error("a non-power of 2 slice size should  have failed")
		return
	}
	for i := 0; i < 16; i++ {
		available, _ = sm.Set(T(i), V(1))
	}
	sm.Set(15, 1)
	added, available, err = sm.BookAndSet(1)
	if err != nil {
		t.Error("unable to call BookAndSet", err)
	} else if added != 16 {
		t.Error("result should be 16", added)
	}
	if compareto[0] != int(available) {
		t.Error("remaining should be 32751", available)
	}
	added, available, err = sm.BookAndSet(1)
	if err != nil {
		t.Error("unable to call BookAndSet", err)
	} else if added != 17 {
		t.Error("result should be 16", added)
	}
	sm.Unset(12)
	added, available, err = sm.BookAndSet(1)
	if err != nil {
		t.Error("unable to call BookAndSet", err)
	} else if added != 12 {
		t.Error("result should be 12", added)
	}
	if compareto[1] != int(available) {
		t.Error("remaining should be 32750", available)
	}

	for i := 0; i < 28000; i++ {
		sm.Set(T(i), V(i))
	}
	added, available, err = sm.BookAndSet(1)
	v := 28000
	if err != nil {
		t.Error("unable to call BookAndSet", err)
	} else if added != T(v) {
		t.Error("result should be 28000", added)
	}
	if compareto[2] != int(available) {
		t.Error("remaining should be 4767", available)
	}
	for _, v := range []uint16{14789, 14790, 17791, 21111} {
		sm.Unset(T(v))
	}
	for _, v := range []uint16{14789, 14790, 17791, 21111} {
		added, available, err = sm.BookAndSet(1)
		if err != nil {
			t.Error("unable to call BookAndSet", err)
		} else if added != T(v) {
			t.Errorf("result should be %d, but got %d", v, added)
		}
	}
	for i := 0; i < sliceSize; i++ {
		sm.Set(T(i), V(i))
	}
	if compareto[3] != int(available) {
		t.Error("remaining should be 4767", available)
	}
	v = 200
	_, available, err = sm.BookAndSet(V(v))
	if err == nil {
		t.Error("should have errored out calling BookAndSet on a full set", err)
	} else if !strings.HasPrefix(err.Error(), "SlotMachine: No ") {
		t.Error("result should be no available|usable slot message", err)
	}
	sm.Unset(0)
	v = 201
	added, available, err = sm.BookAndSet(V(v))
	if err != nil {
		t.Error("unable to call BookAndSet", err)
	} else if added != 0 {
		t.Error("result should be 0", added)
	}

	sm2, _ := New[T, V](
		SyncConcurrency,
		workSlice,
		0,
		uint8(bucketSize),
		boundaries)
	for i := 0; i < 1000; i++ {
		available, _ = sm2.Set(T(i), V(i))
	}
	if compareto[4] != int(available) {
		t.Error("remaining should be 65536", available)
	}
	var addedbunch []T
	addedbunch, available, err = sm2.BookAndSetBatch(5, V(v))
	if err != nil {
		t.Error("unable to call BookAndSetBatch", err)
	} else {
		for i := 0; i < 5; i++ {
			if int(addedbunch[i]) != 1000+i {
				t.Error("result should be 1000 + i", addedbunch[i])
			}
		}
	}
	if compareto[5] != int(available) {
		t.Error("remaining should be 31763", available)
	}
}

func (s *SMTester[T, V]) DefaultTest(t *testing.T, sliceSize int, bucketSize int, boundaries *Boundaries) {
	workSlice := make([]V, sliceSize)
	s.Exercise(t, &workSlice, sliceSize, bucketSize, boundaries)
}

func TestBucketSizeIs2(t *testing.T) {
	t.Log("Testing a bucket size of 2")
	tester := SMTester[uint16, uint16]{}
	tester.DefaultTest(t, 32768, 2, nil)
}

func TestBucketSizeIs8(t *testing.T) {
	t.Log("Testing a bucket size of 8")
	tester := SMTester[uint16, uint16]{}
	tester.DefaultTest(t, 32768, 8, nil)
}

func TestBucketSizeIs14(t *testing.T) {
	t.Log("Testing a bucket size of 14 (must fail)")
	tester := SMTester[uint32, uint16]{}
	tester.DefaultTest(t, 65536, 14, nil)
}

func TestBucketSizeIs16(t *testing.T) {
	t.Log("Testing a bucket size of 16")
	tester := SMTester[uint32, uint16]{}
	tester.DefaultTest(t, 60000, 16, nil)
}

func TestBucketSizeIs16Boundaries(t *testing.T) {
	t.Log("Testing a bucket size of 16")
	tester := SMTester[uint32, uint16]{}
	tester.DefaultTest(t, 65536, 16, &Boundaries{0, 50000})
}

func testConcurrent(t *testing.T, sm SlotMachine[uint32, uint16], threads int, batchSize int) {
	var wg sync.WaitGroup
	var available uint
	var err error
	wg.Add(threads)
	whatevs := 0
	for i := 0; i < threads; i++ {
		go func(idx int) {
			defer wg.Done()
			var slotid uint32
			counter := 0
			allocated := []uint32{}
			for i := 0; i < batchSize; i++ {
				slotid, available, err = sm.BookAndSet(uint16(i))
				allocated = append(allocated, slotid)
				if err != nil {
					t.Error(err)
				}
				t.Logf("Available == %d (also, arbitrary value = %d)", available, whatevs)
				counter++
				whatevs++
			}
			for i := 0; i < len(allocated); i++ {
				available, err = sm.Unset(allocated[i])
				if err != nil {
					t.Error(err)
				}
				t.Logf("Available == %d (also, arbitrary value = %d)", available, whatevs)
			}
		}(i)
	}
	wg.Wait()
	t.Logf("Final Available == %d", available)
}

func TestBucketSizeIs8ConcurrentSync(t *testing.T) {
	t.Log("Testing a bucket size of 8, in a highly concurrent environment (sync)")

	workSlice := make([]uint16, 524288)
	sm, err := New[uint32, uint16](
		SyncConcurrency,
		&workSlice,
		0,
		uint8(8),
		&Boundaries{0, 520000})
	if err != nil {
		t.Error(err)
		return
	}

	start := time.Now()
	testConcurrent(t, sm, 50000, 10)
	t.Logf("Time elapsed: %s", time.Since(start))
}

func TestBucketSizeIs8ConcurrentChan(t *testing.T) {
	t.Log("Testing a bucket size of 8, in a highly concurrent environment (channel)")

	workSlice := make([]uint16, 524288)
	sm, err := New[uint32, uint16](
		ChannelConcurrency,
		&workSlice,
		0,
		uint8(8),
		&Boundaries{0, 520000})
	if err != nil {
		t.Error(err)
		return
	}

	start := time.Now()
	testConcurrent(t, sm, 50000, 10)
	t.Logf("Time elapsed: %s", time.Since(start))
}

func TestBucketSizeIs8Sequential(t *testing.T) {
	t.Log("Testing a bucket size of 8, in a sequential environment, for reference")

	workSlice := make([]uint16, 524288)
	sm, err := New[uint32, uint16](
		NoConcurrency,
		&workSlice,
		0,
		uint8(8),
		&Boundaries{0, 520000})
	if err != nil {
		t.Error(err)
		return
	}

	start := time.Now()
	testConcurrent(t, sm, 1, 500000)
	t.Logf("Time elapsed: %s", time.Since(start))
}

func TestBucketSizeIs8Pretend(t *testing.T) {
	t.Log("Testing a bucket size of 8, in a pretend environment, where I just count, for shame")

	workSlice := make([]uint16, 524288)

	start := time.Now()
	whatevs := 0
	for i := 0; i < 50000; i++ {
		for j := 0; j < 10; j++ {
			workSlice[i*j] = 1
			if i*j > 2 {
				if workSlice[i*j] == workSlice[i*j-1]*workSlice[i*j-2]+1 {
					whatevs++
				}
			}
		}
	}
	t.Logf("Time elapsed: %s (also, arbitrary output: %d)", time.Since(start), whatevs)
}
