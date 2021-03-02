#!/bin/bash

# remove previous generated key-pair (this is for the bchd1 gRPC connection)
#rm /data/rpc.*

# start bchd
bchd --notls -C /data/bchd.conf