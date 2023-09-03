package slotmachine

import (
	"golang.org/x/exp/constraints"
	"testing"
)

type SMTester[T constraints.Integer, V constraints.Integer] struct {
}

func (s *SMTester[T, V]) Exercise(t *testing.T, workSlice *[]V, sliceSize int, bucketSize int, boundaries *Boundaries) {
	sm, err := New[T, V](
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
		sm.Set(T(i), V(1))
	}
	added, err := sm.SyncBookAndSet(2)
	if err != nil {
		t.Error("unable to call SyncBookAndSet", err)
	} else if added != 16 {
		t.Error("result should be 16", added)
	}
	added, err = sm.SyncBookAndSet(3)
	if err != nil {
		t.Error("unable to call SyncBookAndSet", err)
	} else if added != 17 {
		t.Error("result should be 16", added)
	}
	sm.Unset(12)
	added, err = sm.SyncBookAndSet(4)
	if err != nil {
		t.Error("unable to call SyncBookAndSet", err)
	} else if added != 12 {
		t.Error("result should be 12", added)
	}

	for i := 0; i < 28000; i++ {
		sm.Set(T(i), V(i))
	}
	added, err = sm.SyncBookAndSet(100)
	v := 28000
	if err != nil {
		t.Error("unable to call SyncBookAndSet", err)
	} else if added != T(v) {
		t.Error("result should be 28000", added)
	}
	for _, v := range []uint16{14789, 14790, 17791, 21111} {
		sm.SyncUnset(T(v))
	}
	for _, v := range []uint16{14789, 14790, 17791, 21111} {
		added, err = sm.SyncBookAndSet(101)
		if err != nil {
			t.Error("unable to call SyncBookAndSet", err)
		} else if added != T(v) {
			t.Errorf("result should be %d, but got %d", v, added)
		}
	}
	for i := 0; i < sliceSize; i++ {
		sm.Set(T(i), V(i))
	}
	v = 200
	_, err = sm.SyncBookAndSet(V(v))
	if err == nil {
		t.Error("should have errored out calling SyncBookAndSet on a full set", err)
	} else if err.Error() != "SlotMachine: No available slot" && err.Error() != "SlotMachine: No usable slot" {
		t.Error("result should be no available|usable slot message", err)
	}
	sm.Unset(0)
	v = 201
	added, err = sm.SyncBookAndSet(V(v))
	if err != nil {
		t.Error("unable to call SyncBookAndSet", err)
	} else if added != 0 {
		t.Error("result should be 0", added)
	}
	sm.DumpLayout()

	sm2, _ := New[T, V](
		workSlice,
		0,
		uint8(bucketSize),
		boundaries)
	for i := 0; i < 1000; i++ {
		sm2.Set(T(i), V(i))
	}
    addedbunch, err := sm2.SyncBookAndSetBatch(5, V(v))
	if err != nil {
		t.Error("unable to call SyncBookAndSetBatch", err)
    } else {
        for i := 0; i < 5; i++ {
            if int(addedbunch[i]) != 1000 + i {
                t.Error("result should be 1000 + i", addedbunch[i])
            }
        }
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

// TODO coroutines
