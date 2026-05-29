FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
COPY services/control-plane ./services/control-plane

RUN go build -o /out/control-plane ./services/control-plane

FROM alpine:3.22

RUN adduser -D -H -u 10001 controlplane

WORKDIR /app

COPY --from=build /out/control-plane /usr/local/bin/control-plane

USER controlplane
EXPOSE 8080

CMD ["control-plane"]
