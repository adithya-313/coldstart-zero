# Stage 1: Build the Go binary
FROM golang:1.21-alpine AS builder

# Set the working directory inside the builder container
WORKDIR /src

# Copy the Go module files to download dependencies first (for layer caching)
COPY go.mod go.sum* ./

# Copy the application source code
COPY main.go .

# Compile the Go binary with CGO disabled for a fully static binary
# GOOS=linux ensures the binary is built for the Linux operating system
RUN CGO_ENABLED=0 GOOS=linux go build -o /coldstart-api main.go

# Stage 2: Create a minimal scratch image
FROM scratch

# Copy the compiled static binary from the builder stage
COPY --from=builder /coldstart-api /coldstart-api

# Set the binary as the entrypoint when the container starts
ENTRYPOINT ["/coldstart-api"]
