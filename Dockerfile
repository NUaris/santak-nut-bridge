FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY *.go ./
ARG VERSION=0.1.0
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/santak-nut-bridge .

FROM alpine:3.22
RUN addgroup -S ups && adduser -S -G ups ups
COPY --from=build /out/santak-nut-bridge /usr/local/bin/santak-nut-bridge
EXPOSE 3493/tcp
# USB/serial device permissions vary on NAS appliances, so the compose file runs as root.
ENTRYPOINT ["/usr/local/bin/santak-nut-bridge"]
