# Multi-stage: static Go binary on distroless, runs as nonroot.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO_ENABLED=0 keeps it static (modernc.org/sqlite is pure Go).
RUN CGO_ENABLED=0 GOFLAGS=-buildvcs=false go build -o /server .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /server /server
ENV PORT=8788 DATA_DIR=/data
EXPOSE 8788
VOLUME ["/data"]
USER nonroot
ENTRYPOINT ["/server"]
