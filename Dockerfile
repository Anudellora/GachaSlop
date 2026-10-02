FROM node:24-alpine AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gachaslop-api ./cmd/api

FROM alpine:3.22
RUN addgroup -S gachaslop && adduser -S -G gachaslop gachaslop \
    && mkdir -p /app/data && chown -R gachaslop:gachaslop /app
WORKDIR /app
COPY --from=build /out/gachaslop-api /app/gachaslop-api
COPY --from=frontend /src/web/dist /app/web/dist
ENV HTTP_ADDR=0.0.0.0:8080 APP_ENV=production
USER gachaslop
EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["/app/gachaslop-api"]
