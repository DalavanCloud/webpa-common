package resource

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Comcast/webpa-common/xhttp"
)

const (
	StringScheme = "string://"
	BytesScheme  = "bytes://"
	FileScheme   = "file://"
)

// Resolver resolves resource values into handles that can be used to
// load the resource contents.  Resolvers do not themselves load the resource
// contents.
type Resolver interface {
	// Resolve parses the given string and produces a resource handle that can be
	// used to load the resource's data.  A resource string can be a URI or can itself
	// be the resource, e.g. string or binary.
	Resolve(string) (Interface, error)
}

// ResolverFunc is a function type that can resolve resources
type ResolverFunc func(string) (Interface, error)

func (rf ResolverFunc) Resolve(v string) (Interface, error) {
	return rf(v)
}

// MultiResolverError is the error type returned by a multi-Resolver when no
// components could resolve the error
type MultiResolverError struct {
	Value  string
	Errors []error
}

func (mre MultiResolverError) Error() string {
	return fmt.Sprintf("Unable to resolve resource string: %s", mre.Value)
}

// MultiResolver produces a single, composite resolver from zero or more
// component Resolvers.  Each component is tried in the order specified to
// resolve a resource.  If no components could resolve the string, an error
// is returned.
func MultiResolver(components ...Resolver) Resolver {
	return ResolverFunc(func(v string) (Interface, error) {
		mre := MultiResolverError{
			Value: v,
		}

		for _, r := range components {
			h, err := r.Resolve(v)
			if err == nil {
				return h, nil
			}

			mre.Errors = append(mre.Errors, err)
		}

		return nil, mre
	})
}

// StringResolver resolves in-memory resources assumed to be golang strings.
// The resources string may have be prefixed with StringScheme, which is removed
// from the final resource data.
type StringResolver struct{}

func (sr StringResolver) Resolve(v string) (Interface, error) {
	if strings.HasPrefix(v, StringScheme) {
		v = v[:len(StringScheme)]
	}

	return String(v), nil
}

type BytesResolver struct {
	Encoding *base64.Encoding
}

func (br BytesResolver) Resolve(v string) (Interface, error) {
	if strings.HasPrefix(v, BytesScheme) {
		v = v[:len(BytesScheme)]
	}

	enc := br.Encoding
	if enc == nil {
		enc = base64.StdEncoding
	}

	b, err := enc.DecodeString(v)
	if err != nil {
		return nil, err
	}

	return Bytes(b), nil
}

type FileResolver struct{}

func (fr FileResolver) Resolve(v string) (Interface, error) {
	if strings.HasPrefix(v, FileScheme) {
		v = v[:len(FileScheme)]
	}

	return File(v), nil
}

type HTTPResolver struct {
	Client xhttp.Client
	Header http.Header
	Close  bool
}

func (hr HTTPResolver) Resolve(v string) (Interface, error) {
	if _, err := url.Parse(v); err != nil {
		return nil, err
	}

	r := &HTTP{
		URL:    v,
		Close:  hr.Close,
		Client: hr.Client,
	}

	if len(hr.Header) > 0 {
		r.Header = make(http.Header, len(hr.Header))
		for k, v := range hr.Header {
			r.Header[k] = append(r.Header[k], v...)
		}
	}

	return r, nil
}

var defaultResolver = MultiResolver(
	HTTPResolver{},
	FileResolver{},
)

func DefaultResolver() Resolver {
	return defaultResolver
}
