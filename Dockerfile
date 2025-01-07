# Stage 1: Builder
FROM golang:1.23.4-alpine3.20 AS builder
WORKDIR /app

RUN apk add --no-cache git

COPY . .

RUN go build -ldflags="-w -s" -o Earnify .

# Stage 2: Final image
FROM alpine:3.20.2

RUN apk --no-cache add ca-certificates && \
    apk update && apk upgrade --available && sync

COPY --from=builder /app/Earnify /Earnify

ENTRYPOINT ["/Earnify"]
