package opentracing

import (
	"fmt"

	ot "github.com/opentracing/opentracing-go"
	zipkin "github.com/openzipkin/zipkin-go"
	"github.com/openzipkin/zipkin-go/model"
)

type tracerImpl struct {
	tracer           *zipkin.Tracer
	textPropagator   *textMapPropagator
	binaryPropagator *binaryPropagator
}

// NewTracer returns a new OpenTracing tracer based on our native Zipkin Go
// tracer.
func NewTracer(tracer *zipkin.Tracer) ot.Tracer {
	return &tracerImpl{
		tracer: tracer,
	}
}

func (t *tracerImpl) StartSpan(operationName string, opts ...ot.StartSpanOption) ot.Span {
	var (
		otSpanOptions ot.StartSpanOptions
		otBaggage     map[string]string
		spanOptions   []zipkin.SpanOption
		spanContext   model.SpanContext
		defaultTags   = make(map[string]string)
	)

	for _, opt := range opts {
		opt.Apply(&otSpanOptions)
	}

	if !otSpanOptions.StartTime.IsZero() {
		spanOptions = append(spanOptions, zipkin.StartTime(otSpanOptions.StartTime))
	}

	// TODO: we only support one parent at the moment
	for _, ref := range otSpanOptions.References {
		switch ref.Type {
		case ot.ChildOfRef:
			refCtx := ref.ReferencedContext.(spanContextImpl)
			spanContext = refCtx.SpanContext
			otBaggage = refCtx.baggage
			break
		case ot.FollowsFromRef:
			refCtx := ref.ReferencedContext.(spanContextImpl)
			spanContext = refCtx.SpanContext
			otBaggage = refCtx.baggage
			// we override with next occurence in the hope we find a childOf ref
		}
	}

	spanOptions = append(spanOptions, zipkin.Parent(spanContext))
	for k, v := range otSpanOptions.Tags {
		// TODO: do idiomatic conversion of keys
		defaultTags[k] = fmt.Sprintf("%+v", v)
	}

	if len(defaultTags) > 0 {
		spanOptions = append(spanOptions, zipkin.Tags(defaultTags))
	}

	return &spanImpl{
		tracer:  t,
		span:    t.tracer.StartSpan(operationName, spanOptions...),
		baggage: otBaggage,
	}
}

func (t *tracerImpl) Inject(sc ot.SpanContext, format interface{}, carrier interface{}) error {
	switch format {
	case ot.TextMap, ot.HTTPHeaders:
		return t.textPropagator.Inject(sc, carrier)
	case ot.Binary:
		return t.binaryPropagator.Inject(sc, carrier)
	}
	return ot.ErrUnsupportedFormat
}

func (t *tracerImpl) Extract(format interface{}, carrier interface{}) (ot.SpanContext, error) {
	switch format {
	case ot.TextMap, ot.HTTPHeaders:
		return t.textPropagator.Extract(carrier)
	case ot.Binary:
		return t.binaryPropagator.Extract(carrier)
	}
	return nil, ot.ErrUnsupportedFormat
}
