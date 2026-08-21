FROM node:24.19.0-bookworm-slim@sha256:3638d9a6fe4030bd716be989438248074489337ba3275657f93595428be4fc03 AS web-build

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts
COPY web/ ./
RUN npm run build

FROM golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS build

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

FROM gcr.io/distroless/static-debian12:nonroot@sha256:1b7b9f0f0e0a1d2155f531db587cc48ec26aaf97ab64364225f5bf18a054e66a

COPY --from=build /out/finance-core /finance-core

EXPOSE 8000
ENTRYPOINT ["/finance-core"]
CMD ["serve"]
