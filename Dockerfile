FROM golang:1.26-bookworm AS builder

WORKDIR /src

ARG GO_MODULE_PROXY=https://goproxy.cn,direct
ARG GO_CHECKSUM_DB=sum.golang.google.cn
ARG GO_PRIVATE=git.example.com/*

RUN go env -w GOPROXY=${GO_MODULE_PROXY} \
  && go env -w GOSUMDB=${GO_CHECKSUM_DB} \
  && go env -w GOPRIVATE=${GO_PRIVATE} \
  && go env -w GONOSUMDB=${GO_PRIVATE}

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG BRANCH=main
ARG COMMIT=local
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w \
  -X gitee.com/super_sky/mkh_utils.Version=${VERSION} \
  -X gitee.com/super_sky/mkh_utils.Branch=${BRANCH} \
  -X gitee.com/super_sky/mkh_utils.Commit=${COMMIT} \
  -X gitee.com/super_sky/mkh_utils.BuildTime=${BUILD_TIME}" \
  -o /out/athena .

FROM debian:bookworm-slim AS runtime

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /out/athena /app/athena
COPY config /app/config

RUN mkdir -p /app/config/controlplane /srv/shared

EXPOSE 8080

ENTRYPOINT ["/app/athena"]
CMD ["api-server"]
