package app

import "net/http"

func WebHandler() http.Handler {
	return http.HandlerFunc(notFound)
}
