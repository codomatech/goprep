FROM golang:1.21-alpine as gobuilder
MAINTAINER Codoma.tech Advanced Technologies <info@codoma.tech>

COPY ./src /app
WORKDIR /app

RUN go mod vendor
RUN GOOS=linux \
    GOARCH=amd64 \
    CGO_ENABLED=0 \
    go build \
      -mod=vendor \
      -a \
      -trimpath \
      -installsuffix cgo \
      -ldflags="-w -s" \
      -o /bin/goprep

RUN apk add upx && upx -9 /bin/goprep

RUN ls -lh /bin/goprep

FROM  scratch
COPY --from=gobuilder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=gobuilder /bin/goprep /bin/goprep

ENTRYPOINT ["/bin/goprep"]
CMD ["/bin/goprep"]
