package period

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEntries_GetJSON_NotFull(t *testing.T) {
	e := newEntries[int](time.Second)
	now := time.Now().Add(e.interval)
	e.Add(now, 1)
	e.Add(now.Add(time.Second), 2)
	e.Add(now.Add(2*time.Second), 3)
	jsonBytes, err := e.GetJSON()
	require.NoError(t, err)

	expectedJSON := `[1,2,3]`
	require.Equal(t, expectedJSON, string(jsonBytes))
}

func TestEntries_GetJSON_Full(t *testing.T) {
	e := newEntries[int](time.Second)
	now := time.Now().Add(e.interval)
	const exceed = 50
	for i := range maxEntries + exceed {
		e.Add(now.Add(time.Duration(i)*e.interval), i)
	}
	jsonBytes, err := e.GetJSON()
	require.NoError(t, err)

	var expectedJSON bytes.Buffer
	expectedJSON.WriteByte('[')
	// 50 ... 99
	for i := range maxEntries - exceed {
		expectedJSON.WriteString(fmt.Sprintf("%d,", e.entries[maxEntries-exceed+i]))
	}
	// 0 ... 49
	for i := range exceed {
		expectedJSON.WriteString(fmt.Sprintf("%d,", e.entries[i]))
	}
	expectedJSON.Truncate(expectedJSON.Len() - 1) // remove the last comma
	expectedJSON.WriteByte(']')
	require.Equal(t, expectedJSON.String(), string(jsonBytes))
}
