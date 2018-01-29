package opentracing

import (
	"fmt"
	"time"

	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	zipkin "github.com/openzipkin/zipkin-go"
)

type spanImpl struct {
	tracer  ot.Tracer
	span    zipkin.Span
	baggage map[string]string
}

type setFinishTime interface {
	FinishWithTime(time.Time)
}

func (s *spanImpl) Finish() {
	if span, ok := s.span.(setFinishTime); ok {
		span.FinishWithTime(time.Now())
	}
}

func (s *spanImpl) FinishWithOptions(opts ot.FinishOptions) {
	if span, ok := s.span.(setFinishTime); ok {
		finishTime := opts.FinishTime
		if opts.FinishTime.IsZero() {
			finishTime = time.Now()
		}

		for _, lr := range opts.LogRecords {
			s.span.Annotate(lr.Timestamp, fmt.Sprintf("%+v", lr.Fields))
		}
		for _, ld := range opts.BulkLogData {
			s.span.Annotate(ld.Timestamp, fmt.Sprintf("%+v", ld.Event))
		}

		span.FinishWithTime(finishTime)
	}
}

func (s *spanImpl) Context() ot.SpanContext {
	return &spanContextImpl{
		SpanContext: s.span.Context(),
		baggage:     s.baggage,
	}
}

func (s *spanImpl) SetOperationName(operationName string) ot.Span {
	s.span.SetName(operationName)
	return s
}

func (s *spanImpl) SetTag(key string, value interface{}) ot.Span {
	s.span.Tag(key, fmt.Sprintf("%+v", value))
	return s
}

func (s *spanImpl) LogFields(fields ...log.Field) {
	s.span.Annotate(time.Now(), fmt.Sprintf("%+v", fields))
}

func (s *spanImpl) LogKV(alternatingKeyValues ...interface{}) {
	s.span.Annotate(time.Now(), fmt.Sprintf("%+v", alternatingKeyValues))
}

func (s *spanImpl) SetBaggageItem(restrictedKey, value string) ot.Span {
	// TODO: baggage not implemented
	return s
}

func (s *spanImpl) BaggageItem(restrictedKey string) string {
	// TODO: baggage not implemented
	return ""
}

func (s *spanImpl) Tracer() ot.Tracer {
	return s.tracer
}

func (s *spanImpl) LogEvent(event string) {
	s.span.Annotate(time.Now(), event)
}

func (s *spanImpl) LogEventWithPayload(event string, payload interface{}) {
	s.span.Annotate(time.Now(), event)
}

func (s *spanImpl) Log(data ot.LogData) {
	s.span.Annotate(data.Timestamp, data.Event)
}
