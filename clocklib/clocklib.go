package clocklib

import (
	"time"
)

type ClockManagerAPI interface {
	GetCurrentTime() time.Time
}

type ClockManager struct {
	Offset time.Duration
}

func (m *ClockManager) GetCurrentTime() time.Time{
	return time.Now().Add(m.Offset)
}

