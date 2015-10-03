package forward

import (
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	//	"strconv"
	"time"

	"../utils"
)

type Forwarder struct {
	log          utils.Logger
	errHandler   utils.ErrorHandler
	roundTripper http.RoundTripper
	next         http.Handler
}

type optSetter func(f *Forwarder) error

func RoundTripper(r http.RoundTripper) optSetter {
	return func(f *Forwarder) error {
		f.roundTripper = r
		return nil
	}
}

func Logger(l utils.Logger) optSetter {
	return func(f *Forwarder) error {
		f.log = l
		return nil
	}
}

func New(next http.Handler, setters ...optSetter) (*Forwarder, error) {
	f := &Forwarder{
		log:          utils.NullLogger,
		errHandler:   utils.DefaultHandler,
		roundTripper: http.DefaultTransport,
		next:         next,
	}
	for _, s := range setters {
		if err := s(f); err != nil {
			return nil, err
		}
	}
	return f, nil
}

func (f *Forwarder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	reqStr, _ := httputil.DumpRequest(req, true)
	f.log.Debugf("Forward Middleware received request:\n%s", reqStr)

	start := time.Now().UTC()
	reqClone := f.cloneRequest(req, req.URL)

	reqStr, _ = httputil.DumpRequest(reqClone, true)
	f.log.Debugf("Forward Middleware forwards request:\n%s", reqStr)

	response, err := f.roundTripper.RoundTrip(reqClone)

	// TMP
	//f.log.Debugf("******* ORIGINAL :%s\n", req)
	//f.log.Debugf("******* CLONE :%s\n", reqClone)

	if err != nil {
		f.log.Errorf("Error forwarding to %v, err: %v", req.URL, err)
		f.errHandler.ServeHTTP(w, req, err)
		return
	}

	if req.TLS != nil {
		f.log.Infof("Round trip: %v, code: %v, duration: %v tls:version: %x, tls:resume:%t, tls:csuite:%x, tls:server:%v\n",
			req.URL, response.StatusCode, time.Now().UTC().Sub(start),
			req.TLS.Version,
			req.TLS.DidResume,
			req.TLS.CipherSuite,
			req.TLS.ServerName)
	} else {
		f.log.Infof("Round trip: %v, code: %v, duration: %v\n",
			req.URL, response.StatusCode, time.Now().UTC().Sub(start))
	}

	respStr, _ := httputil.DumpResponse(response, true)
	f.log.Debugf("Forward Middleware received response:\n%s", respStr)

	copyHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)

	n, _ := io.Copy(w, response.Body)
	f.log.Debugf("TODO: Byte counting: %v\n", n)
	response.Body.Close()
}

func (f *Forwarder) cloneRequest(req *http.Request, u *url.URL) *http.Request {
	outReq := new(http.Request)
	// Beware, this will make a shallow copy. We have to copy all maps
	*outReq = *req

	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1
	// Overwrite close flag: keep persistent connection for the backend servers
	outReq.Close = false

	// Request Header
	outReq.Header = make(http.Header)
	copyHeaders(outReq.Header, req.Header)

	// Request URL
	var scheme string
	if req.TLS == nil {
		scheme = "http"
	} else {
		scheme = "https"
	}
	outReq.URL = cloneURL(req.URL)
	outReq.URL.Scheme = scheme
	outReq.URL.Host = req.Host
	outReq.URL.Opaque = req.RequestURI
	// raw query is already included in RequestURI, so ignore it to avoid dupes
	outReq.URL.RawQuery = ""
	// Do not pass client Host header unless optsetter PassHostHeader is set.
	return outReq
}

// copyHeaders copies http headers from source to destination.  It does not
// overide, but adds multiple headers
func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// cloneURL provides update safe copy by avoiding shallow copying User field
func cloneURL(i *url.URL) *url.URL {
	out := *i
	if i.User != nil {
		out.User = &(*i.User)
	}
	return &out
}
