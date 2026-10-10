FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/revenueleak ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -S app && mkdir -p /app/data && chown -R app /app
WORKDIR /app
COPY --from=build /out/revenueleak ./revenueleak
COPY web ./web
USER app
ENV ADDR=0.0.0.0:8080 DATA_FILE=/app/data/revenueleak.json WEB_DIR=/app/web
EXPOSE 8080
CMD ["/app/revenueleak"]
