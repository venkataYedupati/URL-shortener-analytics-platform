FROM golang:1.23-alpine AS build

WORKDIR /src
RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG APP_TARGET=api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/${APP_TARGET}

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/app /app/app

EXPOSE 8080
ENTRYPOINT ["/app/app"]
