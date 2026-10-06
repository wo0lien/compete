FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /compete . && mkdir /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /compete /compete
COPY --from=build --chown=65532:65532 /data /data
ENV COMPETE_DB=/data/compete.db
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/compete"]
