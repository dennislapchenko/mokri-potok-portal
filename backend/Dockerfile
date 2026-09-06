# Build context is the REPO ROOT, not backend/ — the image carries the frontend.
# One container answers the API and serves the page, so the portal is same-origin
# on its own domain and needs no CORS and no second host.

FROM node:22-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json frontend/.npmrc ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
# No BASE_PATH and no VITE_API_URL: the app is served from the site root and
# talks to /api on its own origin.
RUN npm run build

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=web /web/dist ./internal/httpapi/web
# CGO_ENABLED=0 keeps it static (modernc.org/sqlite is pure Go).
RUN CGO_ENABLED=0 GOFLAGS=-buildvcs=false go build -o /server .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /server /server
ENV PORT=8788 DATA_DIR=/data
EXPOSE 8788
VOLUME ["/data"]
USER nonroot
ENTRYPOINT ["/server"]
