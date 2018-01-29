package zipkin

import (
	"sync/atomic"
	"time"
)

func (s *spanImpl) FinishWithTime(t time.Time) {
	if atomic.CompareAndSwapInt32(&s.mustCollect, 1, 0) {
		s.Duration = t.Sub(s.Timestamp)
		s.tracer.reporter.Send(s.SpanModel)
	}
}
