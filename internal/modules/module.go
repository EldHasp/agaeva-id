// Package modules describes local tasks the Bitrix24 tab can ask for.
// The core process only routes a request to the module that owns it.
package modules

import "context"

// Request is the payload a module receives. Raw is the JSON body.
// HTML is filled for the print contract; other modules may ignore it.
type Request struct {
	HTML string
	Raw  []byte
}

// Response is what the module returns to the browser tab.
type Response struct {
	Status      int
	ContentType string
	Body        []byte
}

// Module is one local task. The core starts it only when a request arrives.
type Module interface {
	Name() string
	Route() string
	Execute(ctx context.Context, req Request) (Response, error)
}
