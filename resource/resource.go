package resource

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/Comcast/webpa-common/xhttp"
)

var (
	// ErrNoSuchURL is returned when a URL-based resource failed an existence check
	ErrNoSuchURL = errors.New("That URL does not exist")
)

// Interface represents a resource handle.  The existence of a resource handle does not necessarily indicate that the
// underlying data exists.
type Interface interface {
	// Location returns an indicator of where this resource's data comes from.  This is not guaranteed to be a unique string.
	// In-memory resources will all typically return the same value from this string.
	Location() string

	// Exists performs a definitive existence check on the underlying resource's data.
	Exists() bool

	// Open returns a reader for reading this resource's data.  Any error that occurred while attempting to open the resource
	// is returned.
	Open() (io.ReadCloser, error)
}

// String represents an in-memory resource whose data is taken from a string.  Resources of this type have no real location.
// The contents of the resource are the string itself.
type String string

func (s String) Location() string {
	return "string"
}

func (s String) Exists() bool {
	return true
}

func (s String) Open() (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader(string(s))), nil
}

func (s String) WriteTo(w io.Writer) (int64, error) {
	count, err := io.WriteString(w, string(s))
	return int64(count), err
}

// Bytes represents an in-memory resource whose data comes from a byte slice
type Bytes []byte

func (b Bytes) Location() string {
	return "bytes"
}

func (b Bytes) Exists() bool {
	return true
}

func (b Bytes) Open() (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewReader([]byte(b))), nil
}

func (b Bytes) WriteTo(w io.Writer) (int64, error) {
	count, err := w.Write([]byte(b))
	return int64(count), err
}

// DecodeBase64 decodes a base64 string into a Bytes resource.  The encoding is optional,
// and if omitted base64.StdEncoding is used.
func DecodeBase64(v string, enc ...*base64.Encoding) (Interface, error) {
	e := base64.StdEncoding
	if len(enc) > 0 && enc[0] != nil {
		e = enc[0]
	}

	b, err := e.DecodeString(v)
	if err != nil {
		return nil, err
	}

	return Bytes(b), nil
}

// File represents a file system resource
type File string

func (f File) Location() string {
	return string(f)
}

func (f File) Exists() bool {
	fi, err := os.Stat(string(f))
	return err == nil && fi != nil && fi.Mode().IsRegular()
}

func (f File) Open() (io.ReadCloser, error) {
	return os.Open(string(f))
}

// HTTP represents an HTTP resource.  Certain aspects of requesting the resource can be customized.
type HTTP struct {
	URL    string
	Header http.Header
	Close  bool
	Client xhttp.Client
}

func (h *HTTP) Location() string {
	return h.URL
}

func (h *HTTP) newRequest(method string) (*http.Request, error) {
	r, err := http.NewRequest(method, h.URL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range h.Header {
		r.Header[k] = v
	}

	r.Close = h.Close
	return r, nil
}

func (h *HTTP) client() xhttp.Client {
	if h.Client != nil {
		return h.Client
	}

	return http.DefaultClient
}

func (h *HTTP) Exists() bool {
	request, err := h.newRequest(http.MethodHead)
	if err != nil {
		return false
	}

	response, err := h.client().Do(request)
	if err != nil {
		return false
	}

	io.Copy(ioutil.Discard, response.Body)
	response.Body.Close()
	return response.StatusCode < 300
}

func (h *HTTP) Open() (io.ReadCloser, error) {
	request, err := h.newRequest(http.MethodGet)
	if err != nil {
		return nil, err
	}

	response, err := h.client().Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode < 300 {
		return response.Body, nil
	}

	io.Copy(ioutil.Discard, response.Body)
	response.Body.Close()
	return nil, err
}
