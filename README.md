# Slot Machine

# What is this?

I wrote this small library for a very specific use case.

I needed:

- A way to get a free network port to bind an application to, quickly
- Support for coroutines
- Allowing near-instant booking, release and slot-finding

# Limitations

Many. The worst one? Expressing the type being stored in a slot. It is awkward, and not compile-time safe.
Therefore, using this librry beyond its "slot available?" scope could prove a bit hazardous.

Short explanation: this is due to Generics not (yet?) supporting declaring types in method signatures.

TODO LOWER BOUNDARY WILL PREVENT ALLOCATING UNTIL FIXED

# Usage

Import:
```
import (
    "github.com/fusion/slotmachine"
)
```

```
go mod tidy
```

Create your slice, and pass it to the Slot Machine.

The second parameter is used to represent an "empty" value in the slice.
It does not impact the algorithm's behavior.

The third parameter is how wide a bucket should be.
This has a direct impact on the number of buckets and layers.
You can play with this setting to achieve maximum performance, based on your slice size.

```
workSlice, err := make([]uint16, 32768)
sm := slotmachine.New[uint16, uint16](
    ChannelConcurrency,
    &workSlice,
    0,
    uint8(bucketSize),
    nil)
```

For performance reasons, the library insists on workSlice's size, as well as bucketSize's value, being powers of 2. However, you may limit your usable slot range using boundaries:
```
workSlice, err := make([]uint16, 65536)
sm := slotmachine.New[uint16, uint16](
    ChannelConcurrency,
    &workSlice,
    0,
    uint8(bucketSize),
    &slotmachine.Boundaries{5000, 50000})
```

Directly booking and setting a slot:
```
sm.Set(uint16(i), 1)
```
Note: you can check that this call was successful, being within pre-defined boundaries, etc., if it returns an error.

Releasing a slot:
```
sm.Unset(uint16(i))
```

Finding and booking a slot:
```
added, err := sm.BookAndSet(2)
```
This call will return an error about the slice being full if you have used all the slots within your defined boundaries.

In the previous examples, I have used ChannelConcurrency as my concurrency model of choice.

In some instances, e.g. when creating a massive number of goroutines, mutexes can go in "starvation mode" due to the active goroutines not holding the mutex.

In other cases, you may need the flexibility of using a simple mutex, and not need channels.

Finally, you may also not need any concurrency management at all.

For these reasons, you can ask the library to follow one of three concurrency models:

- `NoConcurrency`
- `SyncConcurrency`
- `ChannelConcurrency`

Try different concurrency models and pick the one that works best for your use case!

To get a sense of the performance, both processing and storage-wise, that you are getting, based on your settings:
```
sm.DumpLayout()
```
This will display information such as number of layers, number  of buckets per layer, etc.

# FAQ

**Q: How does this work?**

A: The library maintains a reference to your "managed" slice.

It builds several representational layers, increasingly smaller, to create a "path" to the slice's slots.

As each layer's buckets fill up, their parent layers are updated, and fill up as well. This allows us to find an empty slot very fast by avoiding "traffic jams."

This library eschews the use of trees to preserve maximum locality, and thus memory access performance.

**Q: Is this memory efficient?**

A: Somewhat. It could always improve, though.

The only guarantee is that your storage size will be no worse than going from O(N) to O(Nlogn)
