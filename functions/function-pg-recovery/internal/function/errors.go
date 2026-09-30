package function

import "errors"

var (
	errUnknownOperation   = errors.New("unsupported recovery operation")
	errUnresolvedResource = errors.New("required recovery resource is not resolved")
)
