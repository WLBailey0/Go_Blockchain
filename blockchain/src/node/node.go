package nodePackage

import (
    "fmt"
    "net/http"
    "encoding/json"
    "blockchain"
    "bytes"
    "log"
    "strconv"
//    "net"
// uncomment for local mining    "strings"
    "time"
    "sync"
    "io/ioutil"
)

var NODELIST_FILENAME string = "known_nodes.json"

// define the node address structure of ip and port
type NodeAddress struct {
    IpAddr string
    Port int
    LastSeen int64
}

// define the node structure with a list of addresses
// we will add all of the client/server functions to this struct
type Node struct {
    MyAddress NodeAddress
    NodeList []NodeAddress
    HeightChannel chan int
    BlockIndexChannel chan int
    BlockValidateChannel chan bool
    AddBlockChannel chan blockchainPackage.Block
    GetBlockChannel chan blockchainPackage.Block
    NodeListMutex sync.Mutex
}

//***************************************** Generic Functions ************************************************

// a function to sync node lists
func (nodeInstance *Node) SyncNodes () {
    // lock the mutex so this process can't happen in two threads at the same time
    nodeInstance.NodeListMutex.Lock()

    // register this node with the others
    nodeInstance.RegisterNode()

    // get a list of all nodes on the network
    nodeInstance.GetNodeList()

    // get status of all nodes in list and remove them if they're offline
    nodeInstance.GetNodeStatus()
    nodeInstance.RemoveOfflineNodes()

    // write current node list to disk
    nodeInstance.writeToDisk()
    nodeInstance.NodeListMutex.Unlock()
}

// A function to remove duplicate nodes from the list
func removeDuplicateNodes(nodeList []NodeAddress)  []NodeAddress {
    list := []NodeAddress{}
    for _, node := range nodeList {
        found := false
        // check if any entry in list has the same ip and port
        for _, uniqueNode := range list {
            if node.IpAddr == uniqueNode.IpAddr && node.Port == uniqueNode.Port {
                found = true
            }
        }
        if !found {
            list = append(list, node)
        }
    }
    return list
}

// Use publicly available api to find the public IP of this node
func (nodeInstance *Node) GetPublicIP () {
    // this is a temporary way to use local IP for testing. Uncomment for local mining
    /*conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        fmt.Println(err.Error())
        return
    }
    defer conn.Close()
    localAddr := conn.LocalAddr().(*net.UDPAddr).String()

    nodeInstance.MyAddress.IpAddr = strings.Split(localAddr, ":")[0]*/

/* comment this out for local testing */
    url := "https://api.ipify.org?format=text"
    resp, err := http.Get(url)
    if err != nil {
        fmt.Println(err.Error())
        return
    }
    ip, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        fmt.Println(err.Error())
        return
    }
    nodeInstance.MyAddress.IpAddr = string(ip)
}

// A function to remove nodes from the list that haven't been online lately
func (nodeInstance *Node) RemoveOfflineNodes () {

    // remove nodes that haven't been seen for 36000 seconds
    for i := 0; i < len(nodeInstance.NodeList); i++ {
        if time.Now().Unix() - nodeInstance.NodeList[i].LastSeen > 36000 {
            nodeInstance.NodeList[i] = nodeInstance.NodeList[len(nodeInstance.NodeList) - 1] //move to last element
            nodeInstance.NodeList = nodeInstance.NodeList[:len(nodeInstance.NodeList) - 1] //truncate
        }
    }

    // remove this node's own address if it got added in
    for j := 0; j < len(nodeInstance.NodeList); j++ {
        if nodeInstance.NodeList[j].IpAddr == nodeInstance.MyAddress.IpAddr &&
           nodeInstance.NodeList[j].Port == nodeInstance.MyAddress.Port {
            nodeInstance.NodeList[j] = nodeInstance.NodeList[len(nodeInstance.NodeList) - 1] //move to last element
            nodeInstance.NodeList = nodeInstance.NodeList[:len(nodeInstance.NodeList) - 1] //truncate
        }
    }
}

/******************************************** Disk I/O Functions *****************************************/

// A function to attempt to read the current node list from the disk
func (nodeInstance *Node) ReadFromDisk() bool {
    // declare node list as an empty slice
    diskNodeList := []NodeAddress{}

    // try to open the file
    fileData, err := ioutil.ReadFile(NODELIST_FILENAME)
    if err != nil {
        // return so the nodelist will just keep it's previous node list
        return false
    }

    err = json.Unmarshal(fileData, &diskNodeList)
    if err != nil { //we got an error, so the node list was not formatted well
        return false
    }

    // we successfully read in the file, set the node list
    nodeInstance.NodeList = diskNodeList
    return true
}

func (nodeInstance *Node) writeToDisk() {
    jsonNodeList, err := json.Marshal(nodeInstance.NodeList)
    if err != nil {
        fmt.Println(err.Error())
        return
    }
    err = ioutil.WriteFile(NODELIST_FILENAME, jsonNodeList, 0644)
    if err != nil {
        fmt.Println(err.Error())
        return
    }
}

//************************ Client Functions ***********************************

// A client function to notify other nodes when you find a block
func (nodeInstance *Node) AddBlock(block blockchainPackage.Block) bool {
    client := http.Client{
        Timeout: 10 * time.Second,
    }

    rejectList := []NodeAddress{}
    for _, node := range nodeInstance.NodeList {
        // jsonify the block
        jsonBlock := new(bytes.Buffer)
        err := json.NewEncoder(jsonBlock).Encode(block)
        if err != nil {
            continue
        }

        httpAddress := "http://" + node.IpAddr + ":" + strconv.Itoa(node.Port) + "/add-block"
        resp, err := client.Post(httpAddress, "application/json", jsonBlock)
        if err != nil {
            continue
        }
        if resp.StatusCode == 406 {
            // another node rejected our block
            rejectList = append(rejectList, node)
        }
    }

    // make sure we haven't had our block rejected
    if len(rejectList) > (len(nodeInstance.NodeList) / 2) {
        // more than half of other nodes rejected our block, return false
        return false
    } else {
        return true
    }
}

// A client function to get a list of other nodes to mine with
func (nodeInstance *Node) GetNodeList() {
    client := http.Client{
        Timeout: 10 * time.Second,
    }

    // loop through all known nodes and get their node lists
    for _, node := range nodeInstance.NodeList {
        httpAddress := "http://" + node.IpAddr + ":" + strconv.Itoa(node.Port) + "/get-nodes"
        resp, err := client.Get(httpAddress)
        if err != nil {
            continue
        }

        // convert response body to a slice of NodeAddresses
        var nodeAddresses []NodeAddress
        err = json.NewDecoder(resp.Body).Decode(&nodeAddresses)
        if err != nil { //we got an error, so the node list was not formatted well
            continue
        }

        for _, nodeAddress := range nodeAddresses {
              nodeInstance.NodeList = append(nodeInstance.NodeList, nodeAddress)
        }
        // remove duplicates from the node list
        nodeInstance.NodeList = removeDuplicateNodes(nodeInstance.NodeList)
    }
    return
}

// A client function to let other nodes know a new node has joined
func (nodeInstance *Node) RegisterNode () {
    client := http.Client{
        Timeout: 10 * time.Second,
    }

    for _, node := range nodeInstance.NodeList {
        jsonNodeAddr := new(bytes.Buffer)
        err := json.NewEncoder(jsonNodeAddr).Encode(nodeInstance.MyAddress)
        if err != nil {
            return
        }

        httpAddress := "http://" + node.IpAddr + ":" + strconv.Itoa(node.Port) + "/register-node"
        client.Post(httpAddress, "application/json", jsonNodeAddr)
    }
}

// A client function to get the status of all known nodes
func (nodeInstance *Node) GetNodeStatus () {
    client := http.Client{
        Timeout: 10 * time.Second,
    }

    for i, node := range nodeInstance.NodeList {
        httpAddress := "http://" + node.IpAddr + ":" + strconv.Itoa(node.Port) + "/node-status"
        resp, err := client.Get(httpAddress)
        if err != nil {
            continue
        }
        if resp.StatusCode == 200 {
            nodeInstance.NodeList[i].LastSeen = time.Now().Unix()
        }
    }
}

// A client function to get the blockchain height
func (nodeInstance *Node) GetHeight() int {
    client := http.Client{
        Timeout: 10 * time.Second,
    }

    list := []int{}

    for _, node := range nodeInstance.NodeList {
        var height int
        httpAddress := "http://" + node.IpAddr + ":" + strconv.Itoa(node.Port) + "/get-height"
        resp, err := client.Get(httpAddress)
        if err != nil {
            list = append(list, -1)
        } else {
            err = json.NewDecoder(resp.Body).Decode(&height)
            if err != nil { //we got an error, so the node list was not formatted well
                list = append(list, -2)
            } else {
                list = append(list, height)
            }
        }
    }

    // trust the node with the greatest height
    max := -1
    for _, heights := range list {
        if heights > max {
            max = heights
        }
    }
    return max
}

// A client function for requesting a block from another node
func (nodeInstance *Node) GetBlock(index int) blockchainPackage.Block {
    client := http.Client{
        Timeout: 10 * time.Second,
    }

    list := []blockchainPackage.Block{}

    errBlock := blockchainPackage.Block {
        Index: -2,
    }

    for _, node := range nodeInstance.NodeList {
        jsonIndex := new(bytes.Buffer)
        err := json.NewEncoder(jsonIndex).Encode(index)
        if err != nil {
            return errBlock
        }
        var block blockchainPackage.Block
        httpAddress := "http://" + node.IpAddr + ":" + strconv.Itoa(node.Port) + "/get-block"
        resp, err := client.Post(httpAddress, "application/json", jsonIndex)
        if err != nil {
            // this node didn't work, try another
            continue
        } else {
            if resp.StatusCode != 200 {
                continue
            }
            err = json.NewDecoder(resp.Body).Decode(&block)
            if err != nil { //we got an error, so the node list was not formatted well
                list = append(list, errBlock)
            } else {
                list = append(list, block)
            }
        }
    }

    // all blocks might not be the same, so we choose the one that appears most often
    popularBlock := list[0]
    maxCount := 0
    for _, outerBlock := range list {
        tmpCount := 0
        for _, innerBlock := range list {
            if outerBlock == innerBlock {
                tmpCount++
            }
        }
        if tmpCount > maxCount {
            popularBlock = outerBlock
            maxCount = tmpCount
        }
    }
    return popularBlock
}

//************************ Server Functions ***********************************

// a server function to add a block from another miner to the local blockchain
func (nodeInstance *Node) addRemoteBlock(w http.ResponseWriter, req *http.Request) {
    if req.Body == nil {
        http.Error(w, "Please provide a block information", 400)
        return
    }

    // we got a request with a body
    var proposedBlock blockchainPackage.Block
    err := json.NewDecoder(req.Body).Decode(&proposedBlock)
    if err != nil { //we got an error, so the block was not formatted properly
        http.Error(w, err.Error(), 400)
    }

    nodeInstance.AddBlockChannel <- proposedBlock

    result := <-nodeInstance.BlockValidateChannel
    if result {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusNotAcceptable)
    }
}

// a server function to send all of the nodes that this node is aware of
func (nodeInstance *Node) sendNodeList(w http.ResponseWriter, req *http.Request) {
    // encode our list of nodes to json
    jsonNodeList := new(bytes.Buffer)
    err := json.NewEncoder(jsonNodeList).Encode(nodeInstance.NodeList)
    if err != nil { //we got an error, so the block was not formatted properly
        http.Error(w, err.Error(), 400)
    }
    w.Write(jsonNodeList.Bytes())
}

// a server function to add a new node to the local list of nodes
func (nodeInstance *Node) addNode(w http.ResponseWriter, req *http.Request) {
    if req.Body == nil {
        http.Error(w, "Please provide node information", 400)
        return
    }

    var newAddress NodeAddress
    err := json.NewDecoder(req.Body).Decode(&newAddress)
    if err != nil { //we got an error, so the node address was not formatted well
        return
    }

    newAddress.LastSeen = time.Now().Unix()
    nodeInstance.NodeList = append(nodeInstance.NodeList, newAddress)

    // remove duplicates from the node list
    nodeInstance.NodeList = removeDuplicateNodes(nodeInstance.NodeList)
}

// a server function to let other nodes know this node is still online
func (nodeInstance *Node) nodeStatus(w http.ResponseWriter, req *http.Request) {
    w.WriteHeader(http.StatusOK)
}

// a server function to respond with the blockchain height
func (nodeInstance *Node) sendHeight(w http.ResponseWriter, req *http.Request) {
    // send an int to blockchain requesting height
    nodeInstance.HeightChannel <- 0

    // now wait for response
    height := <-nodeInstance.HeightChannel
    jsonHeight := new(bytes.Buffer)
    err := json.NewEncoder(jsonHeight).Encode(height)
    if err != nil { //we got an error, so the block was not formatted properly
        http.Error(w, err.Error(), 400)
    }
    w.Write(jsonHeight.Bytes())
}

// a server function to respond with a block
func (nodeInstance *Node) sendBlock(w http.ResponseWriter, req *http.Request) {

    // the client will request a specific block index, read it from the request
    if req.Body == nil {
        http.Error(w, "Please provide a block index", 400)
        return
    }

    var blockIndex int
    err := json.NewDecoder(req.Body).Decode(&blockIndex)
    if err != nil {
        http.Error(w, "Please provide a block index as an integer", 400)
        return
    }

    // send an int to blockchain requesting height
    nodeInstance.BlockIndexChannel <- blockIndex

    // now wait for response
    block := <-nodeInstance.GetBlockChannel
    jsonBlock := new(bytes.Buffer)
    err = json.NewEncoder(jsonBlock).Encode(block)
    if err != nil { //we got an error, so the block was not formatted properly
        http.Error(w, err.Error(), 400)
    }
    w.Write(jsonBlock.Bytes())
}

// start the http server and bind server functions to "pages"
func (nodeInstance *Node) Server() {
    http.HandleFunc("/add-block", nodeInstance.addRemoteBlock)
    http.HandleFunc("/get-nodes", nodeInstance.sendNodeList)
    http.HandleFunc("/register-node", nodeInstance.addNode)
    http.HandleFunc("/node-status", nodeInstance.nodeStatus)
    http.HandleFunc("/get-height", nodeInstance.sendHeight)
    http.HandleFunc("/get-block", nodeInstance.sendBlock)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
