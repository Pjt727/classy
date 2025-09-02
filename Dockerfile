# Multistage build

# Stage 1: compile the project
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go application
# CGO_ENABLED=0 disables CGO, making the binary statically linked
# GOOS=linux ensures the binary is built for a Linux environment
RUN CGO_ENABLED=0 GOOS=linux go build -o myapp .

# Stage 2: Create the final, minimal image
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/myapp .
# could look into static linking but it would make supporting tests and regular code that reference files
# more difficult
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/server/static ./server/static

EXPOSE 3000

# Set the entrypoint to run your application
CMD ["/app/myapp", "app", "serve"]
