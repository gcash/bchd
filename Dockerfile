# Building stage
# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang as builder

LABEL maintainer="Josh Ellithorpe <quest@mac.com>"

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/gcash/bchd

# Switch to the correct working directory.
WORKDIR /go/src/github.com/gcash/bchd

# Build the code and the cli client.
RUN go install .
RUN go install ./cmd/bchctl

# Symlink the config to /root/.bchd/bchd.conf
# so bchctl requires fewer flags.
RUN mkdir -p /root/.bchd
RUN ln -s /data/bchd.conf /root/.bchd/bchd.conf

# Final stage
FROM gcr.io/distroless/base

LABEL maintainer="Josh Ellithorpe <quest@mac.com>"

# Copy necessary files
COPY --from=builder /go/bin/bchd /usr/local/bin/bchd
COPY --from=builder /go/bin/bchctl /usr/local/bin/bchctl
COPY --from=builder /root /root

# Create the data volume.
VOLUME ["/data"]

# Set the start command. This starts bchd with
# flags to save the blockchain data and the
# config on a docker volume.
ENTRYPOINT ["bchd", "--addrindex", "--txindex", "-b", "/data", "-C", "/data/bchd.conf"]

# Document that the service listens on port 8333.
EXPOSE 8333
