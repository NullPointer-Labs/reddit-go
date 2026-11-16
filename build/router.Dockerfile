FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /router ./cmd/router

FROM alpine:3.22
COPY --from=build /router /router
EXPOSE 8080
ENTRYPOINT ["/router"]