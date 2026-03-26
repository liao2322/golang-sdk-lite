FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY halalcloud ./halalcloud
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/halal-webui ./cmd/webui

FROM gcr.io/distroless/static-debian12:nonroot

ENV HALAL_API_HOST=openapi.2dland.cn
ENV HALAL_WEB_ADDR=:8080
ENV HALAL_WEB_LINK_MODE=redirect
ENV HALAL_DEFAULT_ROOT=/

WORKDIR /app

COPY --from=builder /out/halal-webui /app/halal-webui

EXPOSE 8080

ENTRYPOINT ["/app/halal-webui"]
