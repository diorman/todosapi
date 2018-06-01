FROM golang:1.10

WORKDIR ${GOPATH}/src/github.com/diorman/todospoc

COPY . ./
