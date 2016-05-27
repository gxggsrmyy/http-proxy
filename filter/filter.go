package filter

import (
	"net/http"

	"github.com/getlantern/http-proxy/utils"
)

// Filter is a special http.Handler that returns true or false depending on
// whether subsequent handlers should continue.
type Filter interface {
	// ServeHTTP is like the function on http.Handler but also returns true or
	// false depending on whether subsequent handlers should continue. If an error
	// occurred, ServeHTTP should return the original error plus a description
	// for logging purposes.
	ServeHTTP(w http.ResponseWriter, req *http.Request) (ok bool, err error, errdesc string)
}

// filterChain is a chain of filters that implements the http.Handler
// interface.
type filterChain struct {
	filters []Filter
}

// NewChain constructs a new chain of filters that executes the filters in order
// until it encounters a filter that returns false.
func NewChain(filters ...Filter) http.Handler {
	return &filterChain{filters}
}

func (chain *filterChain) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, filter := range chain.filters {
		ok, err, desc := filter.ServeHTTP(w, req)
		if err != nil {
			utils.DefaultHandler.ServeHTTP(w, req, err, desc)
		} else if !ok {
			// Interrupt chain
			return
		}
	}
}
