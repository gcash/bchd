# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

LABEL maintainer="Josh Ellithorpe <quest@mac.com>"

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/gcash/bchd

# Switch to the correct working directory.
WORKDIR /go/src/github.com/gcash/bchd

# Restore vendored packages.
RUN go get -u github.com/golang/dep/cmd/dep
RUN dep ensure

# Build the code.
RUN go build .

# Create the data volume.
VOLUME ["/data"]

# Set the start command. This starts bchd with
# flags to save the blockchain data and the
# config on a docker volume.
ENTRYPOINT ["./bchd", "-b", "/data", "-C", "/data/bchd.conf"]

# Document that the service listens on port 8333.
EXPOSE 8333
