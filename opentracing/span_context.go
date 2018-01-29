package opentracing

import "github.com/openzipkin/zipkin-go/model"

type spanContextImpl struct {
	model.SpanContext
	baggage map[string]string
}

// ForeachBaggageItem belongs to the opentracing.SpanContext interface
func (s spanContextImpl) ForeachBaggageItem(handler func(k, v string) bool) {
	for k, v := range s.baggage {
		if !handler(k, v) {
			break
		}
	}
}
