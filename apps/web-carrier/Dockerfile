# syntax=docker/dockerfile:1
# Carrier office image. Build from the repository root:
# docker build -f apps/web-carrier/Dockerfile .
# Runtime public config is the API Gateway base URL only. No secrets.

FROM node:22-alpine AS builder

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@9.15.0 --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps ./apps
COPY packages ./packages

ARG NUXT_PUBLIC_API_BASE_URL=http://localhost:8080
ENV NUXT_PUBLIC_API_BASE_URL=${NUXT_PUBLIC_API_BASE_URL}

RUN pnpm install --frozen-lockfile
RUN pnpm --filter @freight-platform/web-carrier build

FROM node:22-alpine

ARG BINTRANS_GIT_SHA=
ARG BINTRANS_IMAGE_VERSION=
ARG BINTRANS_IMAGE_SOURCE=https://github.com/f9999737292-bit/Freihgt-platform

LABEL org.opencontainers.image.revision="${BINTRANS_GIT_SHA}" \
      org.opencontainers.image.version="${BINTRANS_IMAGE_VERSION}" \
      org.opencontainers.image.source="${BINTRANS_IMAGE_SOURCE}" \
      org.opencontainers.image.title="bintrans-web-carrier" \
      bintrans.component="web-carrier"

WORKDIR /app

ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=3002

RUN addgroup -S webcarrier && adduser -S webcarrier -G webcarrier

COPY --from=builder --chown=webcarrier:webcarrier /app/apps/web-carrier/.output ./.output

USER webcarrier

EXPOSE 3002

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:3002/api/health || exit 1

CMD ["node", ".output/server/index.mjs"]
