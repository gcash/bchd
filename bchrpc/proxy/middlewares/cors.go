package middlewares

import (
	"net/http"
)

// Overrides CORS headers per request.
func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// programatically check the origin against a map of allowed origins
		/*
			origin := r.Header.Get("origin")
			if len(origin) != 0 {
				if _, exists := alloweOriginsMap[origin]; exists {
					// must be exactly 1 value = host (or * for every host)
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}*/

		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}
