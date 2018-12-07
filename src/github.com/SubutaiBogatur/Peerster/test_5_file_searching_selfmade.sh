#!/bin/bash

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

# We are building network like:
# tl - tm - tr
# \         /
#  bl ---- br
# and then: we share a file on "tl", then download it on "tr" and "bl" and then search for it on "br"

echo "Kill all the peerster processes..."
kill $(ps aux | grep '\.\/[P]eerster' | awk '{print $2}')
sleep 1
echo "Killed"

tlUIPort=12001
trUIPort=12002
blUIPort=12003
brUIPort=12004
tmUIPort=12005

tlPort=5001
trPort=5002
blPort=5003
brPort=5004
tmPort=5005

tlAddr="127.0.0.1:$tlPort"
trAddr="127.0.0.1:$trPort"
blAddr="127.0.0.1:$blPort"
brAddr="127.0.0.1:$brPort"
tmAddr="127.0.0.1:$tmPort"

msg_tl_1="I_am_top_left_!"
msg_tr_1="I_am_top_right_!"
msg_bl_1="I_am_bottom_left_!"
msg_br_1="I_am_bottom_right_!"

sharedFileName="pic1.jpg"
trDownloadedFileName="tr-downloaded-$sharedFileName"
blDownloadedFileName="bl-downloaded-$sharedFileName"
brSearchedDownloadedFileName="br-searched-downloaded-$sharedFileName"

rm *.out
rm Peerster
go build
cd client
rm client
go build
cd ..

./Peerster -name=tl -peers="$tmAddr,$blAddr" -UIPort="$tlUIPort" -gossipAddr="$tlAddr" -rtimer=10000 -noWebserver=false -noAntiEntropy=true > TL.out &
./Peerster -name=tr -peers="$tmAddr,$brAddr" -UIPort="$trUIPort" -gossipAddr="$trAddr" -rtimer=10000 -noWebserver=true  -noAntiEntropy=true > TR.out &
./Peerster -name=bl -peers="$tlAddr,$brAddr" -UIPort="$blUIPort" -gossipAddr="$blAddr" -rtimer=10000 -noWebserver=true  -noAntiEntropy=true > BL.out &
./Peerster -name=br -peers="$blAddr,$trAddr" -UIPort="$brUIPort" -gossipAddr="$brAddr" -rtimer=10000 -noWebserver=true  -noAntiEntropy=true > BR.out &
./Peerster -name=tm -peers="$tlAddr,$trAddr" -UIPort="$tmUIPort" -gossipAddr="$tmAddr" -rtimer=10000 -noWebserver=true  -noAntiEntropy=true > TM.out &

# let the gossipers initialize
sleep 1

# some init messages, so everyone knows each other
./client/client -UIPort="$tlUIPort" -msg="$msg_tl_1"
./client/client -UIPort="$trUIPort" -msg="$msg_tr_1"
./client/client -UIPort="$blUIPort" -msg="$msg_bl_1"
./client/client -UIPort="$brUIPort" -msg="$msg_br_1"
./client/client -UIPort="$tmUIPort" -msg="hahahah_im_the_last_one_in_the_middle_1"
./client/client -UIPort="$tmUIPort" -msg="hahahah_im_the_last_one_in_the_middle_2"
./client/client -UIPort="$tmUIPort" -msg="hahahah_im_the_last_one_in_the_middle_3"
./client/client -UIPort="$tmUIPort" -msg="hahahah_im_the_last_one_in_the_middle_4"
./client/client -UIPort="$tmUIPort" -msg="hahahah_im_the_last_one_in_the_middle_5"

sleep 1
echo "~~~~~~~~~~~~ Initial rumor-mongering done, requesting file sharing ~~~~~~~~~~"
sleep 1


# prepare file to send:
cd _SharedFiles
# sending pic1.jpg, which is ready
#rm "$sharedFileName"
#dd if=/dev/zero of="$sharedFileName"  bs=1010K  count=1 # copy a lot of nulls to file (a bit smaller, than 1M for technical reasons, haha)
#echo "ohohohohohohohohohoh" >> "$sharedFileName"
cd ..

cd _Downloads
rm "*.jpg"
cd ..

# share big file on "tl":
./client/client -UIPort="$tlUIPort" -file="$sharedFileName"

sleep 2

fileHash=$(cat TL.out | grep "^[S]HARED.*$sharedFileName" | awk '{print $6}')
echo "~~~~~~ File shared, got hash: $fileHash, starting downloading ~~~~~~"


# request file on "tr" and "bl":
./client/client -UIPort="$trUIPort" -file="$trDownloadedFileName" -dest="tl" -request="$fileHash"
./client/client -UIPort="$blUIPort" -file="$blDownloadedFileName" -dest="tl" -request="$fileHash"

sleep 3
echo "Downloading on tr & bl from tl should have finished"

echo "Initial tests are running.."
if [ -z "$(diff _SharedFiles/$sharedFileName _Downloads/$trDownloadedFileName 2>&1)" ]; then # err moved to out
    # if output of this command is empty
    echo -e "${GREEN}***PASSED***${NC}"
    echo "File $sharedFileName succesfully transfered from tl to tr and saved at tr"
else
    echo -e "${RED}***FAILED***${NC}"
    echo "Bad output is: $(diff _SharedFiles/$sharedFileName _Downloads/$trDownloadedFileName 2>&1)"
    exit
fi

if [ -z "$(diff _SharedFiles/$sharedFileName _Downloads/$blDownloadedFileName 2>&1)" ]; then # err moved to out
    # if output of this command is empty
    echo -e "${GREEN}***PASSED***${NC}"
    echo "File $sharedFileName succesfully transfered from tl to bl and saved at bl"
else
    echo -e "${RED}***FAILED***${NC}"
    echo "Bad output is: $(diff _SharedFiles/$sharedFileName _Downloads/$blDownloadedFileName 2>&1)"
    exit
fi

sleep 1
echo "Okay, great, first part of tests done, files transfered succesfully to other nodes, now search will be tested..."
sleep 1

./client/client -UIPort="$brUIPort" -keywords="pic"
sleep 1
echo "Okay, searching should have been finished on br"

./client/client -UIPort="$brUIPort" -file="$brSearchedDownloadedFileName" -request="$fileHash"
sleep 2
echo "Okay, downloading should have been finished on br"

if [ -z "$(diff _SharedFiles/$sharedFileName _Downloads/$brSearchedDownloadedFileName 2>&1)" ]; then # err moved to out
    echo -e "${GREEN}***PASSED***${NC}"
    echo "File $sharedFileName succesfully transfered from (tr|bl) to br and saved at br after searching, my congratulations, officer"
    echo -e "${GREEN}***MY CONGRATULATIONS SIR***${NC}"
else
    echo -e "${RED}***FAILED***${NC}"
    echo "Bad output is: $(diff _SharedFiles/$sharedFileName _Downloads/$brSearchedDownloadedFileName 2>&1)"
fi

echo "Kill all the peerster processes..."
kill $(ps aux | grep '\.\/[P]eerster' | awk '{print $2}')
sleep 1
echo "Killed"
