# Build the eventfulranges paint demo (demo/paint) into a static Go binary
# with the browser UI embedded under the `embed` build tag.
#
# The Docker build context is the REPOSITORY ROOT (this directory): the demo
# module replaces github.com/d-led/eventfulranges with the parent module, so
# the whole tree must be available to the build. Both of these work from here:
#
#   docker build -t eventfulranges-app .
#   fly deploy --app eventfulranges-app --flycast --no-public-ips

# ---- stage 1: bundle the browser UI ----------------------------------------
FROM node:22-bookworm-slim AS ui
WORKDIR /src/demo/paint/ui-src
COPY demo/paint/ui-src/package.json demo/paint/ui-src/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY demo/paint/ui-src/ ./
RUN npm run build

# ---- stage 2: compile the Go server ----------------------------------------
FROM golang:1.27-bookworm AS builder
WORKDIR /src
# Pull module metadata first so dependency downloads are cached.
COPY go.work go.work.sum ./
COPY go.mod go.sum ./
COPY demo/go.mod demo/go.sum ./demo/
RUN go mod download
# Copy the source, then the freshly built UI (dist/ is dockerignored).
COPY . .
COPY --from=ui /src/demo/paint/dist ./demo/paint/dist
RUN CGO_ENABLED=0 go build -tags embed -o /run-app ./demo/paint

# ---- stage 3: minimal runtime ----------------------------------------------
FROM alpine:3.21
# Sessions are written under $DATA_DIR (/tmp on Fly). Run as an unprivileged
# user and rely on /tmp's world-writable sticky bit for that directory.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /run-app /run-app
USER appuser
ENTRYPOINT ["/run-app"]
