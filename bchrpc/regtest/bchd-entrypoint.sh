#!/bin/bash

if [ "$1" == "bchd1" ];
then
    openssl req -x509 -newkey rsa:4096 -keyout /data/rpc.bchd1.key -out /data/rpc.bchd1.cert -days 365 -subj "/CN=bchd1" -nodes
    bchd --slpcachemaxsize=1 --slpgraphsearch --connect=bchd2 --grpclisten=0.0.0.0 --rpccert=/data/rpc.bchd1.cert --rpckey=/data/rpc.bchd1.key -C /data/bchd.conf
fi

if [ "$1" == "bchd2" ];
then
    bchd --slpcachemaxsize=1 --notls -C /data/bchd.conf
fi
