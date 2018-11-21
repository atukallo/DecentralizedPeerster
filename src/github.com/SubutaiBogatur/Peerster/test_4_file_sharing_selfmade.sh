#!/bin/bash

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

# We are building network like:
# a - b - c - d
# and then sharing files from a to d to test the system

aUIPort=12001
bUIPort=12002
cUIPort=12003
dUIPort=12004

aPort=5001
bPort=5002
cPort=5003
dPort=5004

aAddr="127.0.0.1:$aPort"
bAddr="127.0.0.1:$bPort"
cAddr="127.0.0.1:$cPort"
dAddr="127.0.0.1:$dPort"

msgA1="Istartsharing!"
msgD1="Readytoreceive!"

go build
cd client
go build
cd ..

./Peerster -name=a -peers="$bAddr"        -UIPort=$aUIPort -gossipAddr=$aAddr -rtimer=100 -noWebserver=false -noAntiEntropy=true > A.out &
./Peerster -name=b -peers="$aAddr,$cAddr" -UIPort=$bUIPort -gossipAddr=$bAddr -rtimer=100 -noWebserver=true  -noAntiEntropy=true > B.out &
./Peerster -name=c -peers="$bAddr,$dAddr" -UIPort=$cUIPort -gossipAddr=$cAddr -rtimer=100 -noWebserver=true  -noAntiEntropy=true > C.out &
./Peerster -name=d -peers="$cAddr"        -UIPort=$dUIPort -gossipAddr=$dAddr -rtimer=100 -noWebserver=true  -noAntiEntropy=true > D.out &

# let the gossipers initialize
sleep 1

# some init messages, so a and d will know about each other (with only rtimer initalization pr
./client/client -UIPort=$aUIPort -msg="LetsKnowEachOther1"
./client/client -UIPort=$dUIPort -msg="LetsKnowEachOther2"
./client/client -UIPort=$aUIPort -msg="LetsKnowEachOther3"
./client/client -UIPort=$dUIPort -msg="LetsKnowEachOther4"














