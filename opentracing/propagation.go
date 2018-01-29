package opentracing

import (
	"encoding/binary"
	"io"
	"strings"

	"github.com/gogo/protobuf/proto"
	ot "github.com/opentracing/opentracing-go"
	"github.com/openzipkin/zipkin-go-opentracing/flag"
	"github.com/openzipkin/zipkin-go-opentracing/wire"
	"github.com/openzipkin/zipkin-go/model"
	"github.com/openzipkin/zipkin-go/propagation/b3"
)

type textMapPropagator struct {
	tracer *tracerImpl
}
type binaryPropagator struct {
	tracer *tracerImpl
}

const (
	prefixBaggage = "ot-baggage-"
)

func (p *textMapPropagator) Inject(
	spanContext ot.SpanContext,
	opaqueCarrier interface{},
) error {
	sc, ok := spanContext.(spanContextImpl)
	if !ok {
		return ot.ErrInvalidSpanContext
	}
	carrier, ok := opaqueCarrier.(ot.TextMapWriter)
	if !ok {
		return ot.ErrInvalidCarrier
	}

	if sc.Debug {
		carrier.Set(b3.Flags, "1")
	} else if sc.Sampled != nil {
		// Debug is encoded as X-B3-Flags: 1. Since Debug implies Sampled,
		// so don't also send "X-B3-Sampled: 1".
		if *sc.Sampled {
			carrier.Set(b3.Sampled, "1")
		} else {
			carrier.Set(b3.Sampled, "0")
		}
	}

	if !sc.TraceID.Empty() && sc.ID > 0 {
		carrier.Set(b3.TraceID, sc.TraceID.String())
		carrier.Set(b3.SpanID, sc.ID.String())
		if sc.ParentID != nil {
			carrier.Set(b3.ParentSpanID, sc.ParentID.String())
		}
	}

	for k, v := range sc.baggage {
		carrier.Set(prefixBaggage+k, v)
	}
	return nil
}

func (p *textMapPropagator) Extract(
	opaqueCarrier interface{},
) (ot.SpanContext, error) {
	carrier, ok := opaqueCarrier.(ot.TextMapReader)
	if !ok {
		return nil, ot.ErrInvalidCarrier
	}

	var (
		hdrTraceID      string
		hdrSpanID       string
		hdrParentSpanID string
		hdrSampled      string
		hdrFlags        string
		decodedBaggage  = make(map[string]string)
	)

	carrier.ForeachKey(func(k, v string) error {
		switch strings.ToLower(k) {
		case b3.TraceID:
			hdrTraceID = v
		case b3.SpanID:
			hdrSpanID = v
		case b3.ParentSpanID:
			hdrParentSpanID = v
		case b3.Sampled:
			hdrSampled = v
		case b3.Flags:
			hdrFlags = v
		default:
			lowercaseK := strings.ToLower(k)
			if strings.HasPrefix(lowercaseK, prefixBaggage) {
				decodedBaggage[strings.TrimPrefix(lowercaseK, prefixBaggage)] = v
			}
		}
		return nil
	})

	spanContext, err := b3.ParseHeaders(
		hdrTraceID, hdrSpanID, hdrParentSpanID, hdrSampled, hdrFlags,
	)
	if err != nil {
		return nil, ot.ErrSpanContextCorrupted
	}

	return spanContextImpl{
		SpanContext: *spanContext,
		baggage:     decodedBaggage,
	}, nil
}

func (p *binaryPropagator) Inject(
	spanContext ot.SpanContext,
	opaqueCarrier interface{},
) error {
	sc, ok := spanContext.(spanContextImpl)
	if !ok {
		return ot.ErrInvalidSpanContext
	}
	carrier, ok := opaqueCarrier.(io.Writer)
	if !ok {
		return ot.ErrInvalidCarrier
	}

	state := wire.TracerState{}
	state.TraceId = sc.TraceID.Low
	state.TraceIdHigh = sc.TraceID.High
	state.SpanId = uint64(sc.ID)
	state.Sampled = (sc.Sampled != nil && *sc.Sampled == true)
	state.BaggageItems = sc.baggage

	// encode the debug bit
	flags := flag.Debug
	if sc.ParentID != nil {
		state.ParentSpanId = uint64(*sc.ParentID)
	} else {
		// root span...
		state.ParentSpanId = 0
		flags |= flag.IsRoot
	}

	// we explicitly inform our sampling state downstream
	flags |= flag.SamplingSet
	if sc.Sampled != nil && *sc.Sampled == true {
		flags |= flag.Sampled
	}
	state.Flags = uint64(flags)

	b, err := proto.Marshal(&state)
	if err != nil {
		return err
	}

	// Write the length of the marshalled binary to the writer.
	length := uint32(len(b))
	if err = binary.Write(carrier, binary.BigEndian, &length); err != nil {
		return err
	}

	_, err = carrier.Write(b)
	return err
}

func (p *binaryPropagator) Extract(
	opaqueCarrier interface{},
) (ot.SpanContext, error) {
	carrier, ok := opaqueCarrier.(io.Reader)
	if !ok {
		return nil, ot.ErrInvalidCarrier
	}

	// Read the length of marshalled binary. io.ReadAll isn't that performant
	// since it keeps resizing the underlying buffer as it encounters more bytes
	// to read. By reading the length, we can allocate a fixed sized buf and read
	// the exact amount of bytes into it.
	var length uint32
	if err := binary.Read(carrier, binary.BigEndian, &length); err != nil {
		return nil, ot.ErrSpanContextCorrupted
	}
	buf := make([]byte, length)
	if n, err := carrier.Read(buf); err != nil {
		if n > 0 {
			return nil, ot.ErrSpanContextCorrupted
		}
		return nil, ot.ErrSpanContextNotFound
	}

	ctx := wire.TracerState{}
	if err := proto.Unmarshal(buf, &ctx); err != nil {
		return nil, ot.ErrSpanContextCorrupted
	}

	flags := flag.Flags(ctx.Flags)
	if flags&flag.Sampled == flag.Sampled {
		ctx.Sampled = true
	}
	// this propagator expects sampling state to be explicitly propagated by the
	// upstream service. so set this flag to indentify to tracer it should not
	// run its sampler in case it is not the root of the trace.
	// flags |= flag.SamplingSet

	parentID := model.ID(ctx.ParentSpanId)

	return spanContextImpl{
		SpanContext: model.SpanContext{
			TraceID:  model.TraceID{Low: ctx.TraceId, High: ctx.TraceIdHigh},
			ID:       model.ID(ctx.SpanId),
			Sampled:  &ctx.Sampled,
			ParentID: &parentID,
		},
		baggage: ctx.BaggageItems,
	}, nil
}
