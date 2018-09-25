package clair

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"google.golang.org/grpc"
)

// Clair defines the client for retriving information from the clair API.
type Clair struct {
	URL      string
	Client   *http.Client
	Logf     LogfCallback
	grpcConn *grpc.ClientConn
}

// LogfCallback is the callback for formatting logs.
type LogfCallback func(format string, args ...interface{})

// Quiet discards logs silently.
func Quiet(format string, args ...interface{}) {}

// Log passes log messages to the logging package.
func Log(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// Opt holds the options for a new clair client.
type Opt struct {
	Debug    bool
	Insecure bool
	Timeout  time.Duration
}

// New creates a new Clair struct with the given URL and credentials.
func New(url string, opt Opt) (*Clair, error) {
	transport := http.DefaultTransport

	grpcOpt := []grpc.DialOption{}

	if opt.Insecure {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}

		grpcOpt = append(grpcOpt, grpc.WithInsecure())
	}

	errorTransport := &ErrorTransport{
		Transport: transport,
	}

	// set the logging
	logf := Quiet
	if opt.Debug {
		logf = Log
	}

	conn, err := grpc.Dial(url, grpcOpt...)
	if err != nil {
		logf("grpc dial %s failed: %v", url, err)
	}

	registry := &Clair{
		URL: url,
		Client: &http.Client{
			Timeout:   opt.Timeout,
			Transport: errorTransport,
		},
		Logf:     logf,
		grpcConn: conn,
	}

	return registry, nil
}

// url returns a clair URL with the passed arguements concatenated.
func (c *Clair) url(pathTemplate string, args ...interface{}) string {
	pathSuffix := fmt.Sprintf(pathTemplate, args...)
	url := fmt.Sprintf("%s%s", c.URL, pathSuffix)
	return url
}

func (c *Clair) getJSON(url string, response interface{}) (http.Header, error) {
	resp, err := c.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.Logf("clair.clair resp.Status=%s", resp.Status)

	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		c.Logf("clair.clair resp.Status=%s, body=%s", resp.Status, response)
		return nil, err
	}

	return resp.Header, nil
}
