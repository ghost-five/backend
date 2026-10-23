# Образ сервера. Зависимостей пока нет, но шаг npm ci оставлен: он появится
# вместе с первым пакетом и будет кешироваться отдельно от исходников.
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
# mkdir — на случай, когда зависимостей ещё нет: npm тогда каталог не создаёт.
RUN (npm ci --omit=dev || npm install --omit=dev) && mkdir -p node_modules

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
ARG GIT_SHA=dev
ENV GIT_SHA=$GIT_SHA
COPY --from=deps /app/node_modules ./node_modules
COPY package.json ./
COPY src ./src
COPY migrations ./migrations
# Своим пользователем: в контейнере нечего делать под root.
USER node
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
CMD ["node", "src/server.js"]
