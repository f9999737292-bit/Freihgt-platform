# syntax=docker/dockerfile:1
# BINTRANS CT staging web-admin — Nuxt 3 production Node server (loopback-only publish via compose).

FROM node:22-alpine AS builder

WORKDIR /app

ARG BINTRANS_GIT_SHA=
ARG BINTRANS_IMAGE_VERSION=
ARG NUXT_PUBLIC_API_BASE_URL=http://127.0.0.1:18080
ARG NUXT_PUBLIC_APP_NAME=Bintrans Freight Platform Admin
ARG NUXT_PUBLIC_DEFAULT_LOCALE=ru-RU
ARG NUXT_PUBLIC_DEFAULT_TENANT_ID=
ARG NUXT_PUBLIC_MOCK_AUTH=false

ENV NUXT_PUBLIC_API_BASE_URL=${NUXT_PUBLIC_API_BASE_URL}
ENV NUXT_PUBLIC_APP_NAME=${NUXT_PUBLIC_APP_NAME}
ENV NUXT_PUBLIC_DEFAULT_LOCALE=${NUXT_PUBLIC_DEFAULT_LOCALE}
ENV NUXT_PUBLIC_DEFAULT_TENANT_ID=${NUXT_PUBLIC_DEFAULT_TENANT_ID}
ENV NUXT_PUBLIC_MOCK_AUTH=${NUXT_PUBLIC_MOCK_AUTH}

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM node:22-alpine

ARG BINTRANS_GIT_SHA=
ARG BINTRANS_IMAGE_VERSION=
ARG BINTRANS_IMAGE_SOURCE=https://github.com/f9999737292-bit/Freihgt-platform

LABEL org.opencontainers.image.revision="${BINTRANS_GIT_SHA}" \
      org.opencontainers.image.version="${BINTRANS_IMAGE_VERSION}" \
      org.opencontainers.image.source="${BINTRANS_IMAGE_SOURCE}" \
      org.opencontainers.image.title="bintrans-staging-web-admin" \
      bintrans.component="web-admin" \
      bintrans.release.sha="${BINTRANS_GIT_SHA}"

WORKDIR /app

ENV NODE_ENV=production
ENV HOST=127.0.0.1
ENV PORT=13000

RUN addgroup -S webadmin && adduser -S webadmin -G webadmin

COPY --from=builder --chown=webadmin:webadmin /app/.output ./.output

USER webadmin

EXPOSE 13000

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:13000/login >/dev/null || exit 1

CMD ["node", ".output/server/index.mjs"]
