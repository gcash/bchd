package middlewares

import (
	"net/http"
)

// Disables caching of our responses.
// If caching of API responses is desired you should always explicitly enable it by setting
// the appropriate 'Cache-Control' header value.
// Sending no header defaults to 'private' which allows browsers to cache responses: https://docs.microsoft.com/en-us/previous-versions/iis/6.0-sdk/ms524721(v=vs.90)?redirectedfrom=MSDN
func NoCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// don't use the ugly long header: Cache-Control: private,no-cache,no-store,max-age=0
		// see https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cache-Control
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
