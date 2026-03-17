FROM golang:1.25.1-alpine3.21 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build

FROM alpine:3.21
LABEL org.opencontainers.image.source=https://code.philainel.pw/philainel/witless-tg
LABEL author=philainel
WORKDIR /app
COPY --from=builder /app/migration /app/migration
COPY --from=builder /app/web /app/web
COPY --from=builder /app/witless-tg /app/
CMD [ "/app/witless-tg" ]
