FROM golang:1.26.6-bookworm AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

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
