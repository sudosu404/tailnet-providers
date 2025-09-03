package period

import (
	"bytes"
	"encoding/json"
	"time"
)

type Entries[T any] struct {
	entries  [maxEntries]T
	index    int
	count    int
	interval time.Duration
	lastAdd  time.Time
}

const maxEntries = 100

func newEntries[T any](duration time.Duration) *Entries[T] {
	interval := max(duration/maxEntries, time.Second)
	return &Entries[T]{
		interval: interval,
		lastAdd:  time.Now(),
	}
}

func (e *Entries[T]) Add(now time.Time, info T) {
	if now.Sub(e.lastAdd) < e.interval {
		return
	}
	e.addWithTime(now, info)
}

// addWithTime adds an entry with a specific timestamp without interval checking.
// This is used internally for reconstructing historical data.
func (e *Entries[T]) addWithTime(timestamp time.Time, info T) {
	e.entries[e.index] = info
	e.index = (e.index + 1) % maxEntries
	if e.count < maxEntries {
		e.count++
	}
	e.lastAdd = timestamp
}

// validateInterval checks if the current interval matches the expected interval for the duration.
// Returns true if valid, false if the interval needs to be recalculated.
func (e *Entries[T]) validateInterval(expectedDuration time.Duration) bool {
	expectedInterval := max(expectedDuration/maxEntries, time.Second)
	return e.interval == expectedInterval
}

// fixInterval recalculates and sets the correct interval based on the expected duration.
func (e *Entries[T]) fixInterval(expectedDuration time.Duration) {
	e.interval = max(expectedDuration/maxEntries, time.Second)
}

func (e *Entries[T]) Get() []T {
	if e.count < maxEntries {
		return e.entries[:e.count]
	}
	res := make([]T, maxEntries)
	copy(res, e.entries[e.index:])
	copy(res[maxEntries-e.index:], e.entries[:e.index])
	return res
}

func (e *Entries[T]) Iter(yield func(entry T) bool) {
	if e.count < maxEntries {
		for _, entry := range e.entries[:e.count] {
			if !yield(entry) {
				return
			}
		}
		return
	}
	for _, entry := range e.entries[e.index:] {
		if !yield(entry) {
			return
		}
	}
	for _, entry := range e.entries[:e.index] {
		if !yield(entry) {
			return
		}
	}
}

func (e *Entries[T]) GetJSON() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, maxEntries*1024))
	je := json.NewEncoder(buf)
	buf.WriteByte('[')
	for entry := range e.Iter {
		if err := je.Encode(entry); err != nil {
			return nil, err
		}
		buf.Truncate(buf.Len() - 1) // remove the \n just added by Encode
		buf.WriteByte(',')
	}
	buf.Truncate(buf.Len() - 1) // remove the last comma
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

func (e *Entries[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"entries":  e.Get(),
		"interval": e.interval,
	})
}

func (e *Entries[T]) UnmarshalJSON(data []byte) error {
	var v struct {
		Entries  []T           `json:"entries"`
		Interval time.Duration `json:"interval"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v.Entries) == 0 {
		return nil
	}
	entries := v.Entries
	if len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}

	// Set the interval first before adding entries.
	e.interval = v.Interval

	// Add entries with proper time spacing to respect the interval.
	now := time.Now()
	for i, info := range entries {
		// Calculate timestamp based on entry position and interval.
		// Most recent entry gets current time, older entries get earlier times.
		entryTime := now.Add(-time.Duration(len(entries)-1-i) * e.interval)
		e.addWithTime(entryTime, info)
	}
	return nil
}
