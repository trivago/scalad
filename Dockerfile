FROM golang:1.14.2 as build-env
COPY . /go/src/github.com/trivago/scalad
WORKDIR /go/src/github.com/trivago/scalad
RUN go get
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /scalad

FROM scratch

# Copy root filesystem
COPY rootfs /
COPY --from=build-env /scalad /
ENTRYPOINT ["/scalad"]
