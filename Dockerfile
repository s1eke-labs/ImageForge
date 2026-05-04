FROM node:24-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.24-alpine AS backend-build

WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend-build /app/frontend/dist ./internal/frontend/dist/
RUN CGO_ENABLED=0 go build -o /imageforge ./cmd/server
RUN CGO_ENABLED=0 go build -o /imageforge-cli ./cmd/cli

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=backend-build /imageforge /usr/local/bin/
COPY --from=backend-build /imageforge-cli /usr/local/bin/
EXPOSE 8020
ENTRYPOINT ["imageforge"]
