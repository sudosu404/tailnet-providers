package memlogger

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/puzpuzpuz/xsync/v4"
	apitypes "github.com/yusing/goutils/apitypes"
	"github.com/yusing/goutils/http/websocket"
)

type logEntryRange struct {
	Start, End int
}

type memLogger struct {
	*bytes.Buffer
	sync.RWMutex

	notifyLock sync.RWMutex
	connChans  *xsync.Map[chan *logEntryRange, struct{}]
	listeners  *xsync.Map[chan []byte, struct{}]
}

type MemLogger io.Writer

const (
	maxMemLogSize         = 16 * 1024
	truncateSize          = maxMemLogSize / 2
	initialWriteChunkSize = 4 * 1024
	writeTimeout          = 10 * time.Second
)

var memLoggerInstance = &memLogger{
	Buffer:    bytes.NewBuffer(make([]byte, maxMemLogSize)),
	connChans: xsync.NewMap[chan *logEntryRange, struct{}](),
	listeners: xsync.NewMap[chan []byte, struct{}](),
}

func GetMemLogger() MemLogger {
	return memLoggerInstance
}

func HandlerFunc() gin.HandlerFunc {
	return memLoggerInstance.ServeHTTP
}

func Events() (<-chan []byte, func()) {
	return memLoggerInstance.events()
}

// Write implements io.Writer.
func (m *memLogger) Write(p []byte) (n int, err error) {
	n = len(p)
	m.truncateIfNeeded(n)

	pos, err := m.writeBuf(p)
	if err != nil {
		// not logging the error here, it will cause Run to be called again = infinite loop
		return n, err
	}

	m.notifyWS(pos, n)
	return n, err
}

func (m *memLogger) ServeHTTP(c *gin.Context) {
	manager, err := websocket.NewManagerWithUpgrade(c)
	if err != nil {
		c.Error(apitypes.InternalServerError(err, "failed to create websocket manager"))
		return
	}

	logCh := make(chan *logEntryRange)
	m.connChans.Store(logCh, struct{}{})

	defer func() {
		manager.Close()
		m.notifyLock.Lock()
		m.connChans.Delete(logCh)
		close(logCh)
		m.notifyLock.Unlock()
	}()

	if err := m.wsInitial(manager); err != nil {
		c.Error(apitypes.InternalServerError(err, "failed to send initial log"))
		return
	}

	m.wsStreamLog(c.Request.Context(), manager, logCh)
}

func (m *memLogger) truncateIfNeeded(n int) {
	m.RLock()
	needTruncate := m.Len()+n > maxMemLogSize
	m.RUnlock()

	if needTruncate {
		m.Lock()
		defer m.Unlock()
		needTruncate = m.Len()+n > maxMemLogSize
		if !needTruncate {
			return
		}

		m.Truncate(truncateSize)
	}
}

func (m *memLogger) notifyWS(pos, n int) {
	if m.connChans.Size() == 0 && m.listeners.Size() == 0 {
		return
	}

	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()

	m.notifyLock.RLock()
	defer m.notifyLock.RUnlock()

	m.connChans.Range(func(ch chan *logEntryRange, _ struct{}) bool {
		select {
		case ch <- &logEntryRange{pos, pos + n}:
			return true
		case <-timeout.C:
			return false
		}
	})

	if m.listeners.Size() > 0 {
		msg := m.Bytes()[pos : pos+n]
		m.listeners.Range(func(ch chan []byte, _ struct{}) bool {
			select {
			case <-timeout.C:
				return false
			case ch <- msg:
				return true
			}
		})
	}
}

func (m *memLogger) writeBuf(b []byte) (pos int, err error) {
	m.Lock()
	defer m.Unlock()
	pos = m.Len()
	_, err = m.Buffer.Write(b)
	return pos, err
}

func (m *memLogger) events() (logs <-chan []byte, cancel func()) {
	ch := make(chan []byte)
	m.notifyLock.Lock()
	defer m.notifyLock.Unlock()
	m.listeners.Store(ch, struct{}{})

	return ch, func() {
		m.notifyLock.Lock()
		defer m.notifyLock.Unlock()
		m.listeners.Delete(ch)
		close(ch)
	}
}

func (m *memLogger) wsInitial(manager *websocket.Manager) error {
	m.Lock()
	defer m.Unlock()

	return manager.WriteData(websocket.TextMessage, m.Bytes(), writeTimeout)
}

func (m *memLogger) wsStreamLog(ctx context.Context, manager *websocket.Manager, ch <-chan *logEntryRange) {
	for {
		select {
		case <-ctx.Done():
			return
		case logRange := <-ch:
			m.RLock()
			msg := m.Bytes()[logRange.Start:logRange.End]
			err := manager.WriteData(websocket.TextMessage, msg, writeTimeout)
			m.RUnlock()
			if err != nil {
				return
			}
		}
	}
}
