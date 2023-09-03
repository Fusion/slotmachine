package slotmachine

import (
	"testing"
)

func defaultTest(t *testing.T, bucketSize int) {
	workSlice := make([]uint16, 32768)
	sm := New[uint16, uint16](
		&workSlice,
		0,
		uint8(bucketSize))
	for i := 0; i < 16; i++ {
		sm.Set(uint16(i), 1)
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
		sm.Set(uint16(i), uint16(i))
	}
	added, err = sm.SyncBookAndSet(100)
	if err != nil {
		t.Error("unable to call SyncBookAndSet", err)
	} else if added != 28000 {
		t.Error("result should be 28000", added)
	}
	for _, v := range []uint16{14789, 14790, 17791, 21111} {
		sm.SyncUnset(v)
	}
	for _, v := range []uint16{14789, 14790, 17791, 21111} {
		added, err = sm.SyncBookAndSet(101)
		if err != nil {
			t.Error("unable to call SyncBookAndSet", err)
		} else if added != v {
			t.Errorf("result should be %d, but got %d", v, added)
		}
	}
	for i := 0; i < 32768; i++ {
		sm.Set(uint16(i), uint16(i))
	}
	_, err = sm.SyncBookAndSet(200)
	if err == nil {
		t.Error("should have errored out calling SyncBookAndSet on a full set", err)
	} else if err.Error() != "SlotMachine: No available slot" {
		t.Error("result should be no available slot message", err)
	}
	sm.Unset(0)
	added, err = sm.SyncBookAndSet(201)
	if err != nil {
		t.Error("unable to call SyncBookAndSet", err)
	} else if added != 0 {
		t.Error("result should be 0", added)
	}
}

func TestBucketSizeIs2(t *testing.T) {
	defaultTest(t, 2)
}

func TestBucketSizeIs8(t *testing.T) {
	defaultTest(t, 8)
}

// TODO coroutines
