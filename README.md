# Gollocators

I hope you are smart enough to know what an allocator is and why someone would need it.

## How to use

 - Create allocator with `New` (`NewArena`, `NewPool`, etc.)
 - `allocator.Malloc` allocates memory and returns `unsafe.Pointer`.
 - `CallocSliced[T](allocator, count)` allocates memory for `count` amount of T and returns it wrapped into slice. Use this slice as an array, that means do not use append or other functions that can change or use internals of the slice.
 - `Memory allocated by the allocators is not guaranteed to be zero initialized, you have initialize it manually.`
 - Use `allocator.FreeAll()` to free everything, allocated by the allocator. 
 - Use `allocator.Destroy()` to free everything, allocated by the allocator and also free allocator.
 - There are also collections included to use them with allocators.
 - Create collections with corresponding `Make` function. `List[T]{}` will not work.
 - `NOTHING here is thread safe.`
 - good luck

Oh, and all tests are written by the ai, and I didn't read them all, just a couple to remove tests for fucking String methods.