FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/pigments-web .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/pigments-web /usr/local/bin/pigments-web
RUN mkdir -p /app/data && chown -R app:app /app
USER app
ENV APP_ADDR=0.0.0.0:8080 DATA_DIR=/app/data
EXPOSE 8080
ENTRYPOINT ["pigments-web", "serve"]
