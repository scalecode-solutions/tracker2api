# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tracker2api ./cmd/server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the binary from builder (migrations are embedded via go:embed)
COPY --from=builder /app/tracker2api .

# Copy data files
COPY --from=builder /app/data ./data

# Create uploads directory
RUN mkdir -p /app/uploads/tracker2

# Expose port
EXPOSE 6062

# Run the binary
CMD ["./tracker2api"]
