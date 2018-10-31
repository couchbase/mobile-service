package mobile_service

import (
	"sync"
	"time"
)

type LastSeenMap struct {

	mutex *sync.Mutex

	lastSeen map[string]time.Time
}

func NewLastSeenMap() *LastSeenMap {

	return &LastSeenMap{
		mutex: &sync.Mutex{},
		lastSeen: map[string]time.Time{},
	}

}

func (l *LastSeenMap) UpdateLastSeen(keypath string) {

	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.lastSeen[keypath] = time.Now()

}


func (l *LastSeenMap) StaleEntries(maxStale time.Duration) (staleEntries []string) {

	l.mutex.Lock()
	defer l.mutex.Unlock()

	staleEntries = []string{}

	for keypath, lastSeenAt := range l.lastSeen {

		if time.Since(lastSeenAt) > maxStale {
			staleEntries = append(staleEntries, keypath)
		}

	}

	return staleEntries
}

func (l *LastSeenMap) DeleteEntries(keys []string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for _, key := range keys {
		delete(l.lastSeen, key)
	}

}

