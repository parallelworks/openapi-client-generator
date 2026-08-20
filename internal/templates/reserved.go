package templates

// ReservedIdentifiers are the exported package-level names the templates always
// declare. A schema whose name lands on one of these is renamed, because Go has
// a single package scope and the collision would not compile. Unexported helpers
// need no entry: a generated type name is always exported.
//
// Keep in sync with the templates.
var ReservedIdentifiers = []string{
	"APIError",
	"APIKeyAuth",
	"AuthProvider",
	"BasicAuth",
	"BearerAuth",
	"CallbackNames",
	"Client",
	"ClientOption",
	"DefaultRetryConfig",
	"ErrBadGateway",
	"ErrBadRequest",
	"ErrConflict",
	"ErrForbidden",
	"ErrGatewayTimeout",
	"ErrInternalServerError",
	"ErrNotFound",
	"ErrServiceUnavailable",
	"ErrTooManyRequests",
	"ErrUnauthorized",
	"FormFile",
	"Middleware",
	"NewClient",
	"PageIterator",
	"ParseCallback",
	"ParseWebhook",
	"ResponseMeta",
	"RetryConfig",
	"RoundTripFunc",
	"WebhookNames",
	"WithAuth",
	"WithDefaultRetry",
	"WithHTTPClient",
	"WithMiddleware",
	"WithResponseCapture",
	"WithRetry",
	"WithUserAgent",
}
