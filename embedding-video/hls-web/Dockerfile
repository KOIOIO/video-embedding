FROM node:22-bookworm-slim AS build

WORKDIR /app
COPY hls-web/package.json hls-web/package-lock.json ./
RUN npm ci
COPY hls-web/ ./
RUN npm run build

FROM nginx:1.27-alpine

ENV API_UPSTREAM=http://api:8081
ENV NGINX_ENVSUBST_FILTER=^API_UPSTREAM$

COPY hls-web/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /app/dist /usr/share/nginx/html
