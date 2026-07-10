FROM golang:1.26-alpine AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
	-trimpath \
	-ldflags="-s -w" \
	-o /out/suz-mock \
	./cmd/suz-mock

FROM alpine:3.20

RUN addgroup -S suzmock && adduser -S -G suzmock suzmock

WORKDIR /app
COPY --from=build /out/suz-mock /app/suz-mock

USER suzmock

ENV SUZ_MOCK_ADDR=:8080
EXPOSE 8080

ENTRYPOINT ["/app/suz-mock"]
