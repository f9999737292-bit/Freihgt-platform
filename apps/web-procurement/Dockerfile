# syntax=docker/dockerfile:1
# NON-PRODUCTION pilot/staging packaging for web-procurement (Freight Cost Intelligence workspace).

FROM node:22-alpine AS builder

RUN corepack enable && corepack prepare pnpm@9.15.0 --activate

WORKDIR /repo

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web-procurement/package.json apps/web-procurement/
COPY packages/i18n/package.json packages/i18n/
COPY packages/ui/package.json packages/ui/

RUN pnpm install --frozen-lockfile --filter @freight-platform/web-procurement...

COPY apps/web-procurement apps/web-procurement
COPY packages/i18n packages/i18n
COPY packages/ui packages/ui

ARG NUXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080
ARG NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=false
ARG NUXT_PUBLIC_MOCK_AUTH=false

ENV NUXT_PUBLIC_API_BASE_URL=${NUXT_PUBLIC_API_BASE_URL}
ENV NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=${NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED}
ENV NUXT_PUBLIC_MOCK_AUTH=${NUXT_PUBLIC_MOCK_AUTH}

RUN pnpm --filter @freight-platform/web-procurement build

FROM node:22-alpine

WORKDIR /app

ENV NODE_ENV=production
ENV NUXT_HOST=0.0.0.0
ENV NUXT_PORT=3005

COPY --from=builder /repo/apps/web-procurement/.output ./.output

EXPOSE 3005
CMD ["node", ".output/server/index.mjs"]
