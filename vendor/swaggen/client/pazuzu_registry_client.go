package client

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"

	strfmt "github.com/go-openapi/strfmt"

	"swaggen/client/features"
)

// Default pazuzu registry HTTP client.
var Default = NewHTTPClient(nil)

// NewHTTPClient creates a new pazuzu registry HTTP client.
func NewHTTPClient(formats strfmt.Registry) *PazuzuRegistry {
	if formats == nil {
		formats = strfmt.Default
	}
	transport := httptransport.New("pazuzu.zalando.net", "/api", []string{"http"})
	return New(transport, formats)
}

// New creates a new pazuzu registry client
func New(transport runtime.ClientTransport, formats strfmt.Registry) *PazuzuRegistry {
	cli := new(PazuzuRegistry)
	cli.Transport = transport

	cli.Features = features.New(transport, formats)

	return cli
}

// PazuzuRegistry is a client for pazuzu registry
type PazuzuRegistry struct {
	Features *features.Client

	Transport runtime.ClientTransport
}

// SetTransport changes the transport on the client and all its subresources
func (c *PazuzuRegistry) SetTransport(transport runtime.ClientTransport) {
	c.Transport = transport

	c.Features.SetTransport(transport)

}
