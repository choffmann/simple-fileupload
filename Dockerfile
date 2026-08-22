#############################################
# Builder go
#############################################
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY . .

ARG VERSION=dev
ARG REVISION=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.revision=${REVISION}" \
    -o simple-fileupload .

#############################################
# Runner go
#############################################
FROM scratch

WORKDIR /
VOLUME [ "/data" ]

# scratch ships no ca bundle, and oidc discovery does https before the server starts
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/simple-fileupload /simple-fileupload

EXPOSE 8080
ENV BASE_URL=
ENV UPLOAD_DIR=
ENV OIDC_ISSUER=
ENV OIDC_CLIENT_ID=
ENV OIDC_CLIENT_SECRET=
ENV SESSION_SECRET=
ENV LOG_LEVEL=
ENV LOG_FORMAT=

ENTRYPOINT [ "/simple-fileupload" ]
