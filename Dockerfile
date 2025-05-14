FROM  golang:1.24.2 AS builder
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o vanity -X github.com/jasonlovesdoggo/jsn.Version={{.Env.VERSION}}

FROM scratch AS production
WORKDIR /prod
COPY --from=builder /build/vanity ./
EXPOSE 2143
CMD ["/prod/vanity", "-config", "(data)/config.toml"]