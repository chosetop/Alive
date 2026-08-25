package httpx

import "context"

// contextKey is unexported so no other package can collide with these keys.
type contextKey struct{ name string }

var requestIDKey = contextKey{name: "request_id"}

// HeaderRequestID is the header carrying the request id in and out.
const HeaderRequestID = "X-Request-ID"

// ContextWithRequestID returns a copy of ctx carrying id.
//
// This helper lives in httpx rather than middleware because both the response
// writer and the middleware need it; putting it in middleware would make httpx
// import middleware and middleware import httpx.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns the request id, or "" when absent.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
