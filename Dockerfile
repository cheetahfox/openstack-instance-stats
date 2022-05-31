FROM golang:alpine3.16 as builder

RUN apk add --no-cache --virtual .build-deps gcc musl-dev openssl git

RUN mkdir /go/src/github.com
RUN mkdir /go/src/github.com/cheetahfox

WORKDIR /go/src/github.com/cheetahfox

RUN git clone https://github.com/cheetahfox/openstack-instance-stats.git

WORKDIR /go/src/github.com/cheetahfox/openstack-instance-stats
RUN go build

FROM alpine3:lastest

COPY --from=builder /go/src/github.com/cheetahfox/openstack-instance-stats . 
CMD ./openstack-instance-stats
