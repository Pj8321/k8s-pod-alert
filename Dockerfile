# syntax=docker/dockerfile:1

FROM golang:1.21 AS builder
WORKDIR /src

# copy module files first (better cache)
COPY go.mod ./
RUN go mod download

# copy source
COPY . .

# build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/k8s-pod-alert ./cmd/main.go

# final image
FROM gcr.io/distroless/base-debian12
COPY --from=builder /out/k8s-pod-alert /k8s-pod-alert
ENTRYPOINT ["/k8s-pod-alert"]
