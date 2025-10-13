package rules

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"net/http"
	"strconv"

	gperr "github.com/yusing/goutils/errs"
	"github.com/yusing/goutils/synk"
)

type ResponseModifier struct {
	w          http.ResponseWriter
	b          []byte // the bytes got from pool
	buf        *bytes.Buffer
	statusCode int
	shared     Cache

	hijacked bool

	errs gperr.Builder
}

type Response struct {
	StatusCode int
	Header     http.Header
}

var pool = synk.GetBytesPoolWithUniqueMemory()

func unwrapResponseModifier(w http.ResponseWriter) *ResponseModifier {
	for {
		switch ww := w.(type) {
		case *ResponseModifier:
			return ww
		case interface{ Unwrap() http.ResponseWriter }:
			w = ww.Unwrap()
		default:
			return nil
		}
	}
}

// GetInitResponseModifier returns the response modifier for the given response writer.
// If the response writer is already wrapped, it will return the wrapped response modifier.
// Otherwise, it will return a new response modifier.
func GetInitResponseModifier(w http.ResponseWriter) *ResponseModifier {
	if rm := unwrapResponseModifier(w); rm != nil {
		return rm
	}
	return NewResponseModifier(w)
}

// GetSharedData returns the shared data for the given response writer.
// It will initialize the shared data if not initialized.
func GetSharedData(w http.ResponseWriter) Cache {
	rm := GetInitResponseModifier(w)
	if rm.shared == nil {
		rm.shared = NewCache()
	}
	return rm.shared
}

// NewResponseModifier returns a new response modifier for the given response writer.
//
// It should only be called once, at the very beginning of the request.
func NewResponseModifier(w http.ResponseWriter) *ResponseModifier {
	b := pool.Get()
	return &ResponseModifier{
		w:   w,
		buf: bytes.NewBuffer(b),
		b:   b,
	}
}

// func (rm *ResponseModifier) Unwrap() http.ResponseWriter {
// 	return rm.w
// }

func (rm *ResponseModifier) WriteHeader(code int) {
	rm.statusCode = code
}

func (rm *ResponseModifier) ResetBody() {
	rm.buf.Reset()
}

func (rm *ResponseModifier) ContentLength() int {
	return rm.buf.Len()
}

func (rm *ResponseModifier) StatusCode() int {
	if rm.statusCode == 0 {
		return http.StatusOK
	}
	return rm.statusCode
}

func (rm *ResponseModifier) Header() http.Header {
	return rm.w.Header()
}

func (rm *ResponseModifier) Response() Response {
	return Response{StatusCode: rm.StatusCode(), Header: rm.Header()}
}

func (rm *ResponseModifier) Write(b []byte) (int, error) {
	return rm.buf.Write(b)
}

// AppendError appends an error to the response modifier
// the error will be formatted as "rule <rule.Name> error: <err>"
//
// It will be aggregated and returned in FlushRelease.
func (rm *ResponseModifier) AppendError(rule Rule, err error) {
	rm.errs.Addf("rule %q error: %w", rule.Name, err)
}

func (rm *ResponseModifier) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rm.w.(http.Hijacker); ok {
		rm.hijacked = true
		return hijacker.Hijack()
	}
	return nil, nil, errors.New("hijack not supported")
}

// FlushRelease flushes the response modifier and releases the resources
// it returns the number of bytes written and the aggregated error
// if there is any error (rule errors or write error), it will be returned
func (rm *ResponseModifier) FlushRelease() (int, error) {
	n := 0
	if !rm.hijacked {
		h := rm.w.Header()
		// for k := range h {
		// 	if strings.EqualFold(k, "content-length") {
		// 		h.Del(k)
		// 	}
		// }
		h.Set("Content-Length", strconv.Itoa(rm.buf.Len()))
		rm.w.WriteHeader(rm.StatusCode())
		nn, werr := rm.w.Write(rm.buf.Bytes())
		n += nn
		if werr != nil {
			rm.errs.Addf("write error: %w", werr)
		}

		// flush the response writer
		if flusher, ok := rm.w.(http.Flusher); ok {
			flusher.Flush()
		} else if errFlusher, ok := rm.w.(interface{ Flush() error }); ok {
			ferr := errFlusher.Flush()
			if ferr != nil {
				rm.errs.Addf("flush error: %w", ferr)
			}
		}
	}

	// release the buffer and reset the pointers
	pool.Put(rm.b)
	rm.b = nil
	rm.buf = nil

	// release the shared data
	if rm.shared != nil {
		rm.shared.Release()
		rm.shared = nil
	}

	return n, rm.errs.Error()
}
