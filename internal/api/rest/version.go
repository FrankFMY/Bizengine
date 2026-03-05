package rest

import "net/http"

func APIVersion(version string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-API-Version", version)
			w.Header().Set("X-Mobile-Compatible", "true")
			next.ServeHTTP(w, r)
		})
	}
}
