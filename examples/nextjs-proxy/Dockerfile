# examples/nextjs-proxy/Dockerfile
#
# Builds the Next.js application only.
# The REP gateway runs as a separate service (see docker-compose.yml).
#
# Build:
#   docker build -t rep-nextjs-app .
#
# For a full stack, use docker-compose.yml in this directory.

FROM node:22-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* ./
# Install all deps (including devDependencies) — TypeScript is needed for the build
RUN npm ci
COPY . .
RUN npm run build

FROM node:22-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production

# Non-root user
RUN addgroup --system --gid 1001 nodejs && \
    adduser  --system --uid 1001 nextjs

COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=builder --chown=nextjs:nodejs /app/public ./public

USER nextjs

EXPOSE 3000

# Next.js standalone server
CMD ["node", "server.js"]
