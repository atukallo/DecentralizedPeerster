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

sharedFileName="1M_file.txt"
downloadedFileName="downloaded-$sharedFileName"

go build
cd client
go build
cd ..

./Peerster -name=a -peers="$bAddr"        -UIPort="$aUIPort" -gossipAddr="$aAddr" -rtimer=100 -noWebserver=false -noAntiEntropy=true > A.out &
./Peerster -name=b -peers="$aAddr,$cAddr" -UIPort="$bUIPort" -gossipAddr="$bAddr" -rtimer=100 -noWebserver=true  -noAntiEntropy=true > B.out &
./Peerster -name=c -peers="$bAddr,$dAddr" -UIPort="$cUIPort" -gossipAddr="$cAddr" -rtimer=100 -noWebserver=true  -noAntiEntropy=true > C.out &
./Peerster -name=d -peers="$cAddr"        -UIPort="$dUIPort" -gossipAddr="$dAddr" -rtimer=100 -noWebserver=true  -noAntiEntropy=true > D.out &

# let the gossipers initialize
sleep 1

# some init messages, so a and d will know about each other (with only rtimer initalization pr
./client/client -UIPort="$aUIPort" -msg="LetsKnowEachOther1"
./client/client -UIPort="$dUIPort" -msg="LetsKnowEachOther2"
./client/client -UIPort="$aUIPort" -msg="LetsKnowEachOther3"
./client/client -UIPort="$dUIPort" -msg="LetsKnowEachOther4"

sleep 3
echo "~~~~~~ Initial rumor-mongering done, requesting file sharing ~~~~~~"

# prepare file to send:
cd _SharedFiles
rm "$sharedFileName"
dd if=/dev/zero of="$sharedFileName"  bs=1010K  count=1 # copy a lot of nulls to file (a bit smaller, than 1M for technical reasons, haha)
echo "ahahha" >> "$sharedFileName"
cd ..
rm _Downloads/"$downloadedFileName"

# share big file on "a":
./client/client -UIPort="$aUIPort" -file="$sharedFileName"

sleep 3

fileHash=$(cat A.out | grep "^[S]HARED.*$sharedFileName" | awk '{print $6}')
echo "~~~~~~ File shared, got hash: $fileHash, starting downloading ~~~~~~"


# request file on "d":
./client/client -UIPort="$dUIPort" -file="$downloadedFileName" -dest="a" -request="$fileHash"

sleep 2
echo "Downloading should have been finished"


echo "Kill all the peerster processes..."
kill $(ps aux | grep '\.\/[P]eerster' | awk '{print $2}')
sleep 1
echo "Killed"

echo "Tests running.."
if [ -z "$(diff _SharedFiles/$sharedFileName _Downloads/$downloadedFileName) 2>&1" ]; then # err moved to out
    # if output of this command is empty
    echo -e "${GREEN}***PASSED***${NC}"
    echo "File $sharedFileName succesfully transfered from a to d and saved at d as $downloadedFileName"
else
    echo -e "${RED}***FAILED***${NC}"
    echo "Bad output is: $(diff _SharedFiles/$sharedFileName _Downloads/$downloadedFileName)"
fi
