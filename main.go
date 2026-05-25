package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Global variable to store the fixed boot duration in seconds.
// This is calculated once in main() before the server starts listening.
var bootDurationSeconds float64

// Global counter to track the total number of requests to the /predict endpoint.
// This is incremented atomically under a mutex to safely handle concurrent requests.
var predictRequestCount int64

// Mutex to protect the predictRequestCount from concurrent read/write conflicts.
var requestCountMutex sync.Mutex

// healthHandler handles the GET /health endpoint.
// It always returns HTTP 200 with a JSON status indicating the service is healthy.
func healthHandler(responseWriter http.ResponseWriter, httpRequest *http.Request) {
	// Set the response header to indicate JSON content
	responseWriter.Header().Set("Content-Type", "application/json")
	// Write HTTP 200 status code
	responseWriter.WriteHeader(http.StatusOK)
	// Write the JSON response body
	json.NewEncoder(responseWriter).Encode(map[string]string{
		"status": "healthy",
	})
}

// predictHandler handles the GET /predict endpoint.
// It attempts to read the model weights file from /app/weights/dummy-model.bin.
// If the file is missing or unreadable, it returns HTTP 404 with a JSON error.
// If the file exists, it returns the file size in bytes.
func predictHandler(responseWriter http.ResponseWriter, httpRequest *http.Request) {
	// Increment the predict request counter in a thread-safe way
	requestCountMutex.Lock()
	predictRequestCount++
	requestCountMutex.Unlock()

	// Set the response header to indicate JSON content
	responseWriter.Header().Set("Content-Type", "application/json")

	// Define the path to the model weights file
	// The provision.sh script generates this file as dummy-model.bin
	weightsFilePath := "/app/weights/dummy-model.bin"

	// Attempt to get file information for the weights file
	fileInfo, fileError := os.Stat(weightsFilePath)

	// Check if there was an error reading the file
	if fileError != nil {
		// The file does not exist or cannot be accessed.
		// Return HTTP 404 with a clear JSON error message.
		responseWriter.WriteHeader(http.StatusNotFound)
		json.NewEncoder(responseWriter).Encode(map[string]string{
			"error":   "model weights not found",
			"details": "The file /app/weights/dummy-model.bin does not exist or is not accessible.",
		})
		return
	}

	// The file was found successfully.
	// Return HTTP 200 with the file size in bytes.
	responseWriter.WriteHeader(http.StatusOK)
	json.NewEncoder(responseWriter).Encode(map[string]interface{}{
		"status":    "ok",
		"file_size": fileInfo.Size(),
	})
}

// metricsHandler handles the GET /metrics endpoint.
// It returns plain-text Prometheus-formatted metrics without any third-party libraries.
// Metrics exposed:
//   - coldstart_boot_time_seconds: time from process start to listening state
//   - coldstart_http_requests_total: total number of /predict requests received
func metricsHandler(responseWriter http.ResponseWriter, httpRequest *http.Request) {
	// Read the current request count under the mutex to avoid data races
	requestCountMutex.Lock()
	currentCount := predictRequestCount
	requestCountMutex.Unlock()

	// Set the response header to plain text for Prometheus scraping
	responseWriter.Header().Set("Content-Type", "text/plain; charset=utf-8")
	responseWriter.WriteHeader(http.StatusOK)

	// Build the Prometheus-formatted metrics output manually
	// Each metric includes a HELP line (description) and a TYPE line (gauge/counter)
	// bootDurationSeconds is a fixed value computed once in main() before listening
	metricsOutput := fmt.Sprintf(
		"# HELP coldstart_boot_time_seconds Time from process start to listening state\n"+
			"# TYPE coldstart_boot_time_seconds gauge\n"+
			"coldstart_boot_time_seconds %.6f\n"+
			"# HELP coldstart_http_requests_total Total number of HTTP requests to /predict\n"+
			"# TYPE coldstart_http_requests_total counter\n"+
			"coldstart_http_requests_total %d\n",
		bootDurationSeconds,
		currentCount,
	)

	// Write the formatted metrics string to the HTTP response
	fmt.Fprint(responseWriter, metricsOutput)
}

func main() {
	// Record the exact timestamp when the program started for boot time metrics
	programStartTime := time.Now()

	// Register the /health endpoint with the health handler function
	http.HandleFunc("/health", healthHandler)

	// Register the /predict endpoint with the predict handler function
	http.HandleFunc("/predict", predictHandler)

	// Register the /metrics endpoint with the metrics handler function
	http.HandleFunc("/metrics", metricsHandler)

	// Calculate the time taken to boot the server (before ListenAndServe blocks)
	// Store this as a fixed float64 value so /metrics always reports the same number
	bootDuration := time.Since(programStartTime)
	bootDurationSeconds = bootDuration.Seconds()

	// Log that the server is listening and report the boot time
	log.Printf("Server is listening on port 8080 (boot time: %s)", bootDuration)

	// Start the HTTP server on port 8080.
	// ListenAndServe blocks until the server is stopped.
	// If it fails to start, log the fatal error.
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
