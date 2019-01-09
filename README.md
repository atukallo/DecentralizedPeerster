# Peerster application

The project implements a decentralized peer-to-peer application called "*Peerster*". Done as a semeseter project in course on ["*Decentralized Systems Engineering*"](http://edu.epfl.ch/coursebook/en/decentralized-systems-engineering-CS-438) in EPFL University.

Peerster is an application listening for UDP packets on 2 different addresses: *localhost:client\_port* and *ip:peers\_port*. When Peerster is launched on a local machine, 
commands from either client command line interface or webserver can be issued and are received by local peerster on *client\_port*. Peerster
exchanges data with other peers on another address.

Peerster by itself is a concurrent application with a lot of different goroutines. Main work is usually being done in *message-processor* goroutine, though about a dozen 
other goroutines exist doing background work (waiting for timeouts, mining blocks, running webserver, searching/downloading the file and waiting for answers from peers). 
As a result, a great attention was paid to making the code **race-condition-free** and **deadlock-free**. Code uses both channels go-style architecture and also
shared structures protected with locks. Of course I cannot be sure, that code is fully thread-safe,
but a lot of testing was done with a heap of scripts being written and code performed quite well.

Peerster has the following features:
* **Gossiping protocol**: client can initiate gossips and other peers spread them further. Network of peersters is a graph: every peerster has neighbours -- peers, whose ip-addresses
are known to a given node. There also may exist further nodes, from whom peerster has received indirectly some gossips through neigbours. 
So, peerster knows about their existence, but cannot communicate with them directly. 
The former nodes are called "neighbours", latter ones are "origins". When peerster receives a new gossip, it sends it to a random neighbour and then 
they begin rumor-mongering process, where they synchronize their vector-clocks of (origin, latest-issued-gossip) pairs.
* **Private messages**: issuing a gossip announces a message to the whole network. Private messages provide an alternative solution for information delivery. 
When peerster receives a new gossip, it checks its "origin" and associates his neigbour, who forwarded the gossip to peerster, with the last node on the path to "origin". 
As a result, every peerster gets a *next-hop* map, which is used as a routing table when routing private messages. Every message has also a hop-limit to prevent
the message-storm.
* **Filesharing**: client can request local peerster to share locally-stored file. Then the file is represented as a **Merkle tree** -- the file is splitted into chunks of fixed
size (we use 8Kb to fit into one UDP packet) and for every chunk sha-256 hash is calculated. Then all the hashes are concatenated, splitted once again into chunks, and then
meta-hashes are calculated. Procedure is repated until all the concatenated hashes fit into one chunk. Then we get the *root metahash*, which identifies the shared file.
The client can also request to download the file from specified origin with specified metahash. Then downloader firstly gets the chunk with concatenated hashes identified
with root metahash. Then it goes downway the *Merkle tree* and obtains all the chunks restoring the file at the end.
* **Filesearching**: solves the problem of requiring both metahash and origin of the file to initiate downloading. 
The search request is being spread like a gossip, until the node, owning the requested file is found and then file is downloaded from the node.
* **Blockchain filename claiming**: many nodes can share different files with same name, which will result in problems when searching for needed file. To prevent such a problem
blockchain is introduced. Every time a new file is being shared, the node issues a transaction with filename claiming. If filename is valid (ie not duplicated), transaction is
spread all over the network. At the same time every node is constantly mining new blocks. When the block with valid hash (ie havinig some prefix of 0-bits) is mined, all pending transactions are included there and then block is spread all over the network with every node checking block's **proof-of-work**. The longest block chain the node has is considered an official history. The mechanism to resolve block chain forks is also introduced. When transaction is in the longest chain, filename is officially reserved for a given origin and given metahash and file-download requests can be sent directly to origin found in the blockchain without
doing search.

If interested, see header in [Gossiper.go](src/github.com/SubutaiBogatur/Peerster/gossiper/Gossiper.go) file for more information and sketch of program architecture.

