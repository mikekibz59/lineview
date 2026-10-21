FROM golang:1.26.4-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/loader ./cmd/loader
 
FROM alpine:3
COPY --from=build /out/loader /usr/local/bin/loader

ENTRYPOINT ["loader"]
CMD ["-workers", "4"]