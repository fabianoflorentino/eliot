FROM golang:1.24-alpine AS build

WORKDIR /src

COPY . .

RUN go mod download


RUN CGO_DISABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/eliot ./cmd/eliot

FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=build /out/eliot /eliot

ENV GOMAXPROCS=1

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/eliot"]