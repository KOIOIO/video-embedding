FROM golang:1.26.1-bookworm AS build

WORKDIR /src/hengshui-tablet-video-http

COPY hengshui-tablet-video-http/go.mod hengshui-tablet-video-http/go.sum ./
RUN go mod download

COPY hengshui-tablet-video-http/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/httpapi ./cmd/httpapi \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/recboletrainer ./cmd/recboletrainer

FROM jrottenberg/ffmpeg:latest

WORKDIR /app

COPY --from=build /out/httpapi /usr/local/bin/httpapi
COPY --from=build /out/worker /usr/local/bin/worker
COPY --from=build /out/recboletrainer /usr/local/bin/recboletrainer
COPY --from=build /src/hengshui-tablet-video-http/configs ./configs
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo

ENV TZ=Asia/Shanghai
