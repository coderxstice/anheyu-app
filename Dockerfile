FROM alpine:latest

WORKDIR /anheyu

RUN apk update \
    && apk add --no-cache tzdata vips-tools ffmpeg libheif libraw-tools \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

ENV AN_SETTING_DEFAULT_ENABLE_FFMPEG_GENERATOR=true \
    AN_SETTING_DEFAULT_ENABLE_VIPS_GENERATOR=true \
    AN_SETTING_DEFAULT_ENABLE_LIBRAW_GENERATOR=true

ARG TARGETARCH

COPY anheyu-app-linux-${TARGETARCH} ./anheyu-app

COPY default_files ./default-data

COPY entrypoint.sh ./entrypoint.sh

RUN chmod +x ./anheyu-app \
    && chmod +x ./entrypoint.sh

EXPOSE 8091 443

ENTRYPOINT ["./entrypoint.sh"]

CMD ["./anheyu-app"]