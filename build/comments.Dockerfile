FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /comments ./cmd/comments

FROM alpine:3.22
COPY --from=build /comments /comments
EXPOSE 8081
ENTRYPOINT ["/comments"]