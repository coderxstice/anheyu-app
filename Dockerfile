FROM alpine:latest

# 构建参数（版本信息）
ARG VERSION=unknown
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ARG TARGETARCH

# 镜像标签
LABEL org.opencontainers.image.title="Anheyu App"
LABEL org.opencontainers.image.description="Anheyu App - Self-hosted blog and content management system"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${COMMIT}"
LABEL org.opencontainers.image.created="${BUILD_DATE}"
LABEL org.opencontainers.image.source="https://github.com/anzhiyu-c/anheyu-app"
LABEL org.opencontainers.image.url="https://github.com/anzhiyu-c/anheyu-app"
LABEL org.opencontainers.image.documentation="https://github.com/anzhiyu-c/anheyu-app/blob/main/README.md"
LABEL org.opencontainers.image.vendor="AnzhiYu"
LABEL org.opencontainers.image.licenses="MIT"

WORKDIR /anheyu

# 安装系统依赖
RUN apk update \
    && apk add --no-cache tzdata vips-tools ffmpeg libheif libraw-tools \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

# 设置环境变量
ENV AN_SETTING_DEFAULT_ENABLE_FFMPEG_GENERATOR=true \
    AN_SETTING_DEFAULT_ENABLE_VIPS_GENERATOR=true \
    AN_SETTING_DEFAULT_ENABLE_LIBRAW_GENERATOR=true

# GoReleaser v2 多平台构建支持
ARG TARGETPLATFORM

COPY anheyu-app ./anheyu-app

COPY default_files ./default-data

COPY entrypoint.sh ./entrypoint.sh

# 设置执行权限并显示版本信息
RUN chmod +x ./anheyu-app \
    && chmod +x ./entrypoint.sh \
    && echo "🚀 Anheyu App Docker Image Built Successfully!" \
    && echo "📋 Build Information:" \
    && echo "   Version: ${VERSION}" \
    && echo "   Commit:  ${COMMIT}" \
    && echo "   Date:    ${BUILD_DATE}" \
    && echo "   Arch:    ${TARGETARCH}"

EXPOSE 8091 443

ENTRYPOINT ["./entrypoint.sh"]

CMD ["./anheyu-app"]