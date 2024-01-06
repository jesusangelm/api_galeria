FROM golang:1.21.3-alpine3.18 AS build-stage
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s' -o /api ./cmd/api

FROM alpine:3.19 AS build-release-stage
WORKDIR /
COPY --from=build-stage /api /api
EXPOSE 4000
ENTRYPOINT ["/api"]
