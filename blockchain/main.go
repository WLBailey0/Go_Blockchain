package main

import (
    "blockchain"
    "node"
    "time"
    "fmt"
    "strconv"
    "sync"
)

var STARTING_DIFFICULTY string = "0000007fffffffff"
var NUM_BLOCKS int = 300
var PORT int = 8080
var KNOWN_NODES = []nodePackage.NodeAddress{{"192.168.0.251", 8080, time.Now().Unix()},
                                            {"192.168.0.129", 8080, time.Now().Unix()}}

func mineBlocks(blockchainInstance *blockchainPackage.Blockchain, nodeInstance *nodePackage.Node) {
    for len(blockchainInstance.Chain) < NUM_BLOCKS {
        // add the new block to the blockchain
        blockchainInstance.AddBlock()
	if !blockchainInstance.ValidateChain() {
            // remove the most recent block
            blockchainInstance.Chain = blockchainInstance.Chain[:len(blockchainInstance.Chain) - 1]
        }
	fmt.Println("Found block number " + strconv.Itoa(len(blockchainInstance.Chain)))
        if !nodeInstance.AddBlock(blockchainInstance.Chain[len(blockchainInstance.Chain) - 1]) {
            // Our block was rejected by some of the nodes. We may be out of sync
            blockchainInstance.Chain = blockchainInstance.Chain[:len(blockchainInstance.Chain) - 1]
            syncChain(blockchainInstance, nodeInstance)
        }
    }
    blockchainInstance.WriteChain()
    for _, block := range blockchainInstance.Chain {
        fmt.Println(block)
    }
}

func nodeSetup(nodeInstance *nodePackage.Node) {
    // get the public IP of this node
    nodeInstance.GetPublicIP()

    // set port in node address
    nodeInstance.MyAddress.Port = PORT

    // get a list of clients from well known locations
    nodeInstance.GetNodeList()

    // sync nodes
    nodeInstance.SyncNodes()
}

func syncChain(blockchainInstance *blockchainPackage.Blockchain, nodeInstance *nodePackage.Node) {
    // get the height of the blockchain
    height := nodeInstance.GetHeight()

    // sync the blockchain from the other node
    synced := len(blockchainInstance.Chain)
    for synced < height {
        fmt.Println("Syncing block number " + strconv.Itoa(synced + 1))
        newBlock := nodeInstance.GetBlock(synced)
        blockchainInstance.Chain = append(blockchainInstance.Chain, newBlock)
        synced++
    }
}

func main() {

    // start by initializing a single block to avoid range errors in other functions
    genesisBlock := blockchainPackage.Block {
        Index: 0,
        Timestamp: time.Now().Unix(),
        Proof: 69, //nice
        PreviousHash: "this is just a test",
	Difficulty: STARTING_DIFFICULTY,
    }

    // create channels so the blockchain and node packages can communicate
    sharedHeightChannel := make(chan int)
    sharedBlockIndexChannel := make(chan int)
    sharedGetBlockChannel := make(chan blockchainPackage.Block)
    sharedAddBlockChannel := make(chan blockchainPackage.Block)
    sharedBlockValidateChannel := make(chan bool)

    // create mutexes (mutices?) for safety
    var nodeListMutex sync.Mutex
    var blockMutex sync.Mutex

    // create the blockchain instance
    blockchainInstance := blockchainPackage.Blockchain {
        Chain: make([]blockchainPackage.Block, 0),
        HeightChannel: sharedHeightChannel,
        GetBlockChannel: sharedGetBlockChannel,
        AddBlockChannel: sharedAddBlockChannel,
        BlockIndexChannel: sharedBlockIndexChannel,
        BlockValidateChannel: sharedBlockValidateChannel,
        BlockMutex: blockMutex,
    }

    // create the node instance
    nodeInstance := nodePackage.Node {
        // initialize the node list with well known nodes
        HeightChannel: sharedHeightChannel,
        GetBlockChannel: sharedGetBlockChannel,
        AddBlockChannel: sharedAddBlockChannel,
        NodeListMutex: nodeListMutex,
        BlockValidateChannel: sharedBlockValidateChannel,
        BlockIndexChannel: sharedBlockIndexChannel,
    }

    // this is just a test, improve later to make genesis block mined rather than manually created
    if blockchainInstance.ReadChain() == false {
	    blockchainInstance.Chain = append(blockchainInstance.Chain, genesisBlock)
    }

    // try to read a list of known nodes from disk. If that fails, use the KNOWN_NODES variable
    if !nodeInstance.ReadFromDisk() {
        nodeInstance.NodeList = KNOWN_NODES
    }

    // Start the blockchain threads that listen on channels
    go blockchainInstance.SendHeight()
    go blockchainInstance.SendBlocks()
    go blockchainInstance.AddRemoteBlocks()

    nodeSetup(&nodeInstance)

    syncChain(&blockchainInstance, &nodeInstance)

    // start the server now that everything has synced
    go nodeInstance.Server()

    // do the mining in a goroutine
    go mineBlocks(&blockchainInstance, &nodeInstance)

    // every 5 seconds, sync the nodes
    for {
        go nodeInstance.SyncNodes()
        time.Sleep(5000 * time.Millisecond)
    }
}
