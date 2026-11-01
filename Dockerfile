FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY internal ./internal
COPY pkg ./pkg
RUN CGO_ENABLED=0 go build -trimpath -o /out/blog-api main.go

FROM alpine:3.23
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
RUN mkdir /app/logs && chown app:app /app/logs
COPY --from=build /out/blog-api /app/blog-api
COPY migrations /app/migrations
ENV SERVER_PORT=8080 LOG_FILE=/app/logs/log.txt
USER app
EXPOSE 8080
ENTRYPOINT ["/app/blog-api"]
