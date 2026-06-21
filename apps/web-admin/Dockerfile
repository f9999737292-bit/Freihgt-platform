# syntax=docker/dockerfile:1

FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web-admin/package.json ./apps/web-admin/
COPY packages ./packages

RUN corepack enable && corepack prepare pnpm@9.15.0 --activate
RUN pnpm install --frozen-lockfile || pnpm install

WORKDIR /app/apps/web-admin
RUN pnpm build

FROM node:20-alpine

WORKDIR /app

ENV NODE_ENV=production
ENV NUXT_HOST=0.0.0.0
ENV NUXT_PORT=3000

COPY --from=builder /app/apps/web-admin/.output ./.output

EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
