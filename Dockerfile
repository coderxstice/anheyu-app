FROM alpine:latest

WORKDIR /anheyu

RUN apk update \
    && apk add --no-cache tzdata vips-tools ffmpeg libheif libraw-tools \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

ENV AN_SETTING_DEFAULT_ENABLE_FFMPEG_GENERATOR=true \
    AN_SETTING_DEFAULT_ENABLE_VIPS_GENERATOR=true \
    AN_SETTING_DEFAULT_ENABLE_LIBRAW_GENERATOR=true

COPY anheyu-app-linux-arm64 ./anheyu-app-linux-arm64

COPY data ./data

RUN chmod +x ./anheyu-app-linux-arm64

EXPOSE 8091 443

CMD ["./anheyu-app-linux-arm64"]