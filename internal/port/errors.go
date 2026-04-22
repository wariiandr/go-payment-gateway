package port

import "errors"

var (
	ErrMessaging = errors.New("messaging")
	ErrProvider  = errors.New("provider")
)
