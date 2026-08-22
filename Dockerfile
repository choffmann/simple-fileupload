#############################################
# Builder go
#############################################
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o simple-fileupload .

#############################################
# Runner go
#############################################
FROM scratch

WORKDIR /
VOLUME [ "/data" ]

COPY --from=builder /app/simple-fileupload /simple-fileupload

EXPOSE 8080
ENV BASE_URL=
ENV UPLOAD_DIR=
ENV OIDC_ISSUER=
ENV OIDC_CLIENT_ID=
ENV OIDC_CLIENT_SECRET=
ENV SESSION_SECRET=

ENTRYPOINT [ "/simple-fileupload" ]
