package worker

import "time"

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

func RealClock() Clock { return realClock{} }

type fixedClock struct {
	now time.Time
}

func NewFixedClock(now time.Time) Clock {
	return fixedClock{now: now.UTC()}
}

func (c fixedClock) Now() time.Time { return c.now }
