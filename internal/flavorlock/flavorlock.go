package flavorlock

import "sync"

var (
	mu    sync.Mutex
	locks = make(map[string]*sync.Mutex)
)

// returns an unlock function; the mutex stays in the map, dropping it would race a waiter
func Lock(flavor string) func() {
	mu.Lock()
	lock := locks[flavor]
	if lock == nil {
		lock = &sync.Mutex{}
		locks[flavor] = lock
	}
	mu.Unlock()
	lock.Lock()
	return lock.Unlock
}
