FROM golang:1.23.2-alpine
WORKDIR /app

# Copy go.mod and go.sum files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o captionimager

# Expose the application port
EXPOSE 5000

# Set the entrypoint to the compiled binary
ENTRYPOINT ["/app/captionimager"]