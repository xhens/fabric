#!/usr/bin/env bash

# generate orderer binary
go build

# check in the fabric-samples/bin if there is an orderer.bak and delete it
rm $GOPATH/src/github.com/hyperledger/fabric-samples/bin/orderer.bak

# move orderer binary to fabric-samples/bin
mv $GOPATH/src/github.com/hyperledger/fabric-samples/bin/orderer $GOPATH/src/github.com/hyperledger/fabric-samples/bin/orderer.bak

# copy new binary to bin
mv orderer $GOPATH/src/github.com/hyperledger/fabric-samples/bin/orderer
