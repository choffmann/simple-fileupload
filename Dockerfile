#############################################
# Builder go
#############################################
FROM golang:1.23-alpine AS builder

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
ENV BASIC_AUTH_USERNAME=
ENV BASIC_AUTH_PASSWORD=

ENTRYPOINT [ "/simple-fileupload" ]

