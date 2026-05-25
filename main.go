package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

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
// It attempts to read the model weights file from /app/weights/model.bin.
// If the file is missing or unreadable, it returns HTTP 404 with a JSON error.
// If the file exists, it returns the file size in bytes.
func predictHandler(responseWriter http.ResponseWriter, httpRequest *http.Request) {
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
			"details": "The file /app/weights/model.bin does not exist or is not accessible.",
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

func main() {
	// Record the time when the server started for observability
	startTime := time.Now()

	// Register the /health endpoint with the health handler function
	http.HandleFunc("/health", healthHandler)

	// Register the /predict endpoint with the predict handler function
	http.HandleFunc("/predict", predictHandler)

	// Calculate the time taken to boot the server
	bootDuration := time.Since(startTime)

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
