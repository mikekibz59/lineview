FROM golang:1.26.4-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/migrator ./cmd/migrator

FROM alpine:3
COPY --from=build /out/migrator /usr/local/bin/migrator

ENTRYPOINT ["migrator"]
CMD ["-workers", "4"]
