# syntax=docker/dockerfile:1
FROM golang:1.21 as builder
WORKDIR /src
COPY . .
RUN go mod download && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/k8s-pod-event-collector ./cmd/main.go

FROM gcr.io/distroless/base-debian12
COPY --from=builder /out/k8s-pod-event-collector /k8s-pod-event-collector
ENTRYPOINT ["/k8s-pod-event-collector"]
