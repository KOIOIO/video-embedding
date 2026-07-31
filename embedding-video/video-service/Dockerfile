FROM golang:1.26.1-bookworm AS build

WORKDIR /src/video-service

COPY video-service/go.mod video-service/go.sum ./
RUN go mod download

COPY video-service/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/httpapi ./cmd/httpapi \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/recboletrainer ./cmd/recboletrainer

FROM jrottenberg/ffmpeg:latest

WORKDIR /app

COPY --from=build /out/httpapi /usr/local/bin/httpapi
COPY --from=build /out/worker /usr/local/bin/worker
COPY --from=build /out/recboletrainer /usr/local/bin/recboletrainer
COPY --from=build /src/video-service/configs ./configs
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

ENV TZ=Asia/Shanghai
