FROM node:24.19.0-bookworm-slim AS web-build

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts
COPY web/ ./
RUN npm run build

FROM golang:1.26.6-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY --from=web-build /src/web/dist ./internal/webassets/dist

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags='-s -w' \
    -o /out/finance-core \
    ./cmd/finance-core

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/finance-core /finance-core

EXPOSE 8000
ENTRYPOINT ["/finance-core"]
CMD ["serve"]
