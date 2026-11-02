// Command healthcheck performs a small local HTTP readiness probe for a
// container that intentionally ships without a shell or curl binary.
package main

import (
	"context"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(2)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	context, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, endpoint := range os.Args[1:] {
		request, err := http.NewRequestWithContext(context, http.MethodGet, endpoint, nil)
		if err != nil {
			os.Exit(1)
		}
		response, err := client.Do(request)
		if err != nil {
			os.Exit(1)
		}
		response.Body.Close()
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			os.Exit(1)
		}
	}
}
