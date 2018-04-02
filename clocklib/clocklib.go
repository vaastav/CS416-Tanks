package clocklib

import (
	"sync/atomic"
	"time"
)

type ClockManagerAPI interface {
	GetCurrentTime() time.Time
}

type ClockManager struct {
	offset int64
}

func (m *ClockManager) SetOffset(offset time.Duration) {
	atomic.StoreInt64(&m.offset, int64(offset))
}

func (m *ClockManager) GetOffset() time.Duration {
	return time.Duration(atomic.LoadInt64(&m.offset))
}

func (m *ClockManager) GetCurrentTime() time.Time {
	return time.Now().Add(m.GetOffset())
}
