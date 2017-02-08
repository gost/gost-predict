FROM golang:latest
RUN apt-get update
RUN mkdir -p /go/src/github.com/gost/gost-predict/
RUN go get github.com/gorilla/mux
RUN go build -o /go/bin/gost/gost-predict github.com/gost/gost-predict
WORKDIR /go/bin/gost-predict
ENTRYPOINT ["/go/bin/gost/gost-predict"]
EXPOSE 8080