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
    workSlice := make([]uint16, 32768)
    sm := slotmachine.New[uint16, uint16](
        &workSlice,
        0,
        uint8(bucketSize))
```

Directly booking and setting a slot:
```
    sm.Set(uint16(i), 1)
```

Releasing a slot:
```
    sm.Unset(uint16(i))
```

Finding and booking a slot:
```
    added, err := sm.SyncBookAndSet(2)
```

More synchronized calls:
```
    sm.SyncSet(uint16(i), 1)
    sm.SyncUnset(uint16(i))
```

# FAQ

Q: How does this work?

A: The library maintains a reference to your "managed" slice. It builds several representational layers, increasingly smaller, to create a "path" to the slice's slots.
As each layer's buckets fill up, their parent layers are updated, and fill up as well. This allows us to find an empty slot very fast by avoiding "traffic jams."
This library eschews the use of trees to preserve maximum locality, and thus memory access performance.

Q: Is this memory efficient?

A: Somewhat. It could always improve, though. The only guarantee is that your storage size will be no worse than going from O(N) to O(Nlogn)
Note that you can improve memory usage by aligning bucket width to memory words, as I am using bitwise arithmetics.
