# syntax=docker/dockerfile:1

FROM golang:1.21 AS builder

# Match Go module path (this fixes the import issue)
WORKDIR /go/src/k8s-pod-alert

# Copy go.mod and go.sum first
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /k8s-pod-alert ./cmd/main.go

# ---- Final lightweight runtime image ----
FROM gcr.io/distroless/base-debian12
COPY --from=builder /k8s-pod-alert /k8s-pod-alert
ENTRYPOINT ["/k8s-pod-alert"]
