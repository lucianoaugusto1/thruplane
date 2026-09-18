# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/nexoroute ./cmd/nexoroute

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/nexoroute /nexoroute
COPY config.example.yaml /etc/nexoroute/config.yaml
EXPOSE 8080
ENTRYPOINT ["/nexoroute"]
CMD ["-config", "/etc/nexoroute/config.yaml"]
