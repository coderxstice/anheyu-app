FROM alpine:latest

WORKDIR /anheyu

ARG TARGETARCH

ARG TARGETVARIANT

RUN apk update \
    && apk add --no-cache tzdata vips-tools ffmpeg libheif libraw-tools \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

ENV AN_SETTING_DEFAULT_ENABLE_FFMPEG_GENERATOR=true \
    AN_SETTING_DEFAULT_ENABLE_VIPS_GENERATOR=true \
    AN_SETTING_DEFAULT_ENABLE_LIBRAW_GENERATOR=true

COPY anheyu-app-linux-${TARGETARCH}${TARGETVARIANT} ./anheyu-app

COPY data ./data

RUN chmod +x ./anheyu-app

EXPOSE 8091 443

# CMD 现在执行的是统一的文件名
CMD ["./anheyu-app"]