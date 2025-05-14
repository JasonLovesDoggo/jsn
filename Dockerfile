FROM  golang:1.24.2 AS builder
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o vanity -ldflags "-X pkg.jsn.cam/jsn.Version=v0.2.0" ./cmd/pkg.jsn.cam

FROM scratch AS production
WORKDIR /prod
COPY --from=builder /build/vanity ./
EXPOSE 2143
CMD ["/prod/vanity", "-config", "(data)/config.toml"]