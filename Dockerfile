FROM --platform=$BUILDPLATFORM golang:1.27 AS build
ARG TARGETOS TARGETARCH VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
      -ldflags="-s -w -X main.version=$VERSION" -o /compete . && mkdir /data

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.source=https://github.com/wo0lien/compete
COPY --from=build /compete /compete
COPY --from=build --chown=65532:65532 /data /data
ENV COMPETE_DB=/data/compete.db
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/compete"]
