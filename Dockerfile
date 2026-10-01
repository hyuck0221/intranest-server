FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum version ./
RUN go mod download
COPY cmd ./cmd
RUN mkdir -p /out && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X main.version=$(cat /src/version)" -o /out/intranest ./cmd/intranest

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=build /out/intranest /app/intranest
ENV INTRANEST_LISTEN_ADDR=0.0.0.0:8080
ENV INTRANEST_DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]
USER nonroot:nonroot
ENTRYPOINT ["/app/intranest"]
