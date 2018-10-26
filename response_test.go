package typhon

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseWriter(t *testing.T) {
	t.Parallel()
	req := Request{}

	// Using NewResponse, vanilla
	r := NewResponse(req)
	r.Write([]byte("boop"))
	b, _ := r.BodyBytes(true)
	assert.Equal(t, []byte("boop"), b)

	// Using NewResponse, via ResponseWriter
	r = NewResponse(req)
	r.Writer().Header().Set("abc", "def")
	r.Writer().WriteHeader(http.StatusForbidden) // Test some other fun stuff while we're here
	r.Writer().Write([]byte("boop"))
	b, _ = r.BodyBytes(true)
	assert.Equal(t, []byte("boop"), b)
	assert.Equal(t, http.StatusForbidden, r.StatusCode)
	assert.Equal(t, "def", r.Header.Get("abc"))

	// Using NewResponse, vanilla and then ResponseWriter
	r = NewResponse(req)
	r.Write([]byte("boop"))
	r.Writer().Write([]byte("woop"))
	b, _ = r.BodyBytes(true)
	assert.Equal(t, []byte("boopwoop"), b)
}

func TestResponseWriter_Error(t *testing.T) {
	t.Parallel()
	rsp := NewResponse(Request{})
	rsp.Writer().WriteError(errors.New("abc"))
	assert.Error(t, rsp.Error)
	assert.Equal(t, "abc", rsp.Error.Error())
}

// TestResponseDecodeCloses verifies that a response body is closed after calling Decode()
func TestResponseDecodeCloses(t *testing.T) {
	t.Parallel()
	rsp := NewResponse(Request{})
	b := []byte(`{"a":"b"}` + "\n")
	r := newDoneReader(ioutil.NopCloser(bytes.NewReader(b)), -1)
	rsp.Body = r

	bout := map[string]string{}
	assert.NoError(t, rsp.Decode(&bout))
	assert.Equal(t, map[string]string{
		"a": "b"}, bout)
	select {
	case <-r.closed:
	default:
		assert.Fail(t, "response body was not closed after Decode()")
	}
}

// TestResponseDecodeJSON_TrailingSpace verifies that trailing newlines do not result in a decoding error
func TestResponseDecodeJSON_TrailingSpace(t *testing.T) {
	t.Parallel()
	rsp := NewResponse(Request{})
	rsp.Body = ioutil.NopCloser(strings.NewReader(`{"foo":"bar"}` + "\n\n\n\n"))

	bout := map[string]string{}
	assert.NoError(t, rsp.Decode(&bout))
	assert.Equal(t, map[string]string{
		"foo": "bar"}, bout)
}

// rc is a helper type used in tests involving a generic io.ReadCloser
type rc struct {
	strings.Reader
	closed int
}

func (v *rc) Close() error {
	v.closed += 1
	return nil
}

// TestResponseBodyBytes_Consuming verifies that Response.BodyBytes behaves as expected in consuming mode (ie. where it
// is expected that future calls to BodyBytes() will return EOF).
//
// The BodyBytes function is specialised for efficiency on some common types that Typhon uses as a Response.Body; this
// test verifies that these specialisations work as expected along with the general io.ReadCloser case.
func TestResponseBodyBytes_Consuming(t *testing.T) {
	t.Parallel()

	// Most general case: an opaque io.ReadCloser
	body := &rc{*strings.NewReader("abc"), 0}
	rsp := NewResponse(Request{})
	rsp.Body = body
	b, err := rsp.BodyBytes(true)
	require.NoError(t, err)
	assert.Equal(t, []byte("abc"), b)
	assert.Equal(t, 1, body.closed) // The reader should have been closed

	// Specialised case: *bufCloser
	rsp.Body = &bufCloser{*bytes.NewBuffer([]byte("def"))}
	b, err = rsp.BodyBytes(true)
	require.NoError(t, err)
	assert.Equal(t, []byte("def"), b)
}

// TestResponseBodyBytes_Preserving verifies that Response.BodyBytes behaves as expected in consuming mode (ie. where it
// is expected that future calls to BodyBytes() will return EOF).
//
// The BodyBytes function is specialised for efficiency on some common types that Typhon uses as a Response.Body; this
// test verifies that these specialisations work as expected along with the general io.ReadCloser case.
func TestResponseBodyBytes_Preserving(t *testing.T) {
	t.Parallel()

	// Most general case: an opaque io.ReadCloser
	body := &rc{*strings.NewReader("abc"), 0}
	rsp := NewResponse(Request{})
	rsp.Body = body
	for i := 0; i < 10; i++ { // Repeated reads should yield the same result
		b, err := rsp.BodyBytes(false)
		require.NoError(t, err)
		assert.Equal(t, []byte("abc"), b)
		assert.Equal(t, 1, body.closed) // The underlying reader should have been closed exactly once
	}

	// Specialised case: *bufCloser
	rsp.Body = &bufCloser{*bytes.NewBuffer([]byte("def"))}
	for i := 0; i < 100; i++ { // Repeated reads should yield the same result
		b, err := rsp.BodyBytes(false)
		require.NoError(t, err)
		assert.Equal(t, []byte("def"), b)
	}
}
