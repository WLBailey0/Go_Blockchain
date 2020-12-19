package blockchainPackage

import (
    "crypto/sha256"
    "time"
    "strconv"
    "encoding/hex"
    "fmt"
    "strings"
    "math/rand"
    "sync"
    "encoding/json"
    "io/ioutil"
)

var BLOCK_TIME int64 = 120
var BLOCK_ADJUSTMENT int = 720
var NUM_OUTLIERS int = 60
var JSONCHAIN string = "chain_storage.json"

// define the blockchain structure. In go we add the functions for this structure later
type Blockchain struct {
    Chain []Block
    HeightChannel chan int
    BlockIndexChannel chan int
    GetBlockChannel chan Block
    AddBlockChannel chan Block
    BlockValidateChannel chan bool
    BlockMutex sync.Mutex
}

// define the block structure
type Block struct {
    Index int
    Timestamp int64
    Proof int
    PreviousHash string
    Difficulty string
}

// add a function to the blockchain struct to get the previous block
func (bc *Blockchain) GetPreviousBlock() Block {
    return bc.Chain[len(bc.Chain) - 1]
}

// function to print block information, not sure if we'll need long term
func (bc *Blockchain) PrintBlockInfo(index int) {
    block := bc.Chain[index]
    fmt.Println("Index of the block is " + strconv.Itoa(block.Index))
    fmt.Println("Timestamp of the block is " + time.Unix(block.Timestamp, 0).Format(time.UnixDate))
    fmt.Println("Proof of the block is " + strconv.Itoa(block.Proof))
    fmt.Println("Hash of the previous block is " + block.PreviousHash)
    fmt.Println("Hash of the current block is " + bc.HashBlock(block))
    fmt.Println("Difficulty of the block is " + block.Difficulty)
    fmt.Println("\n\n")
}

// Increment hex value by one lexicographically. Used to adjust difficulty 
func hexInc(hash []byte) []byte {
    for i := 0; i < len(hash) -1; i++ {
        val := hash[i]
        if (val == 48) { // this value is a zero
            continue
        } else {
            carry := true
            var start int
            if (val == 102) { // leave it alone if it's an f
                start = i - 1
            } else {
                start = i
            }
            for j := start; j >= 0; j-- {
                val2 := hash[j]
                // a->f
                if val2 > 96 {
                val2 -= 96-9
                } else {
                    val2 -= 48
                }
                if carry {
                    val2 +=1
                    carry = false
                }
                if val2 == 16 {
                    val2 = 0
                    carry = true
                }
                if val2 >= 10 {
                    hash[j] = val2+96-9
                } else {
                    hash[j] = val2+48
                }
            }
            break
        }
    }
    return hash
}

// Decrement the hex value by one lexicographically. Used to adjust difficulty
func hexDec(hash []byte) []byte {
    var r = make([]byte, len(hash))
    carry := true
    for i := 0; i < len(hash); i++ {
        val := hash[i]
        if (val == 48) {
            r[i] = val
            continue
        }
        // a->f
        if val > 96 {
            val -= 96-9
        } else {
            val -= 48
        }
        if carry {
            val -=1
            carry = false
        }
        if (val+1) == 0 {
            val = 15
            carry = true
        }
        if val >= 10 {
            r[i] = val+96-9
        } else {
            r[i] = val+48
        }
    }
    return r
}

// A function to adjust the difficulty based on the average time between
// the last 720 blocks with 120 outliers removed
func (bc *Blockchain) AdjustDifficulty() string {
    // check average time between last 10 blocks
    if (len(bc.Chain) <= BLOCK_ADJUSTMENT) {
        return bc.Chain[0].Difficulty
    } else {
        var timestamps []int64
        for i := len(bc.Chain) - 1; i > len(bc.Chain) - BLOCK_ADJUSTMENT; i-- {
            if (i > 0) {
                timestamps = append(timestamps, bc.Chain[i].Timestamp - bc.Chain[i-1].Timestamp)
            }
        }

        // Take out the highest and lowest OUTLIER_NUM timestamps
        for i := 0; i < NUM_OUTLIERS; i++ {
            // identify the highest and lowest
            var min int64 = 99999999
            var max int64 = -1
            var max_index int = -1
            var min_index int = -1
            for j:= 0; j < len(timestamps); j++ {
                if timestamps[j] > max {
                    max = timestamps[j]
                    max_index = j
                }
                if timestamps[j] < min {
                    min = timestamps[j]
                    min_index = j
                }
            }
            // delete the min and max
            timestamps[min_index] = timestamps[len(timestamps) - 1] //move min to last element
            timestamps[max_index] = timestamps[len(timestamps) - 2] //move max to second to last element
            timestamps = timestamps[:len(timestamps) - 2] //truncate
        }

        // calculate the average after taking out the outliers
        var running_total int64 = 0
        for j := 0; j < len(timestamps); j++ {
            running_total = running_total + timestamps[j]
        }
        average := running_total / int64(len(timestamps))
        b := []byte(bc.Chain[len(bc.Chain) - 1].Difficulty)

        // either increase or decrease the difficulty based on the average
        if (average > BLOCK_TIME) {
            return string(hexInc(b))
        } else {
            return string(hexDec(b))
        }
    }
}

// add a function to the blockchain struct to add a new block
func (bc *Blockchain) AddBlock() {
    newBlock := new(Block)
    newBlock.Proof, newBlock.Timestamp = bc.ProofOfWork()
    //newBlock.Timestamp = time.Now().Unix()
    newBlock.Index = len(bc.Chain)
    newBlock.PreviousHash = bc.HashBlock(bc.Chain[len(bc.Chain) - 1])
    newBlock.Difficulty = bc.AdjustDifficulty()

    bc.BlockMutex.Lock()
    bc.Chain = append(bc.Chain, *newBlock)
    bc.BlockMutex.Unlock()
}

// add a function to the blockchain struct to create a hash
func (bc *Blockchain) HashBlock(block Block) string {
    var hash = sha256.New()
    hash.Write([]byte(strconv.Itoa(block.Index) +
               time.Unix(block.Timestamp, 0).Format(time.UnixDate) +
               strconv.Itoa(block.Proof) +
               block.PreviousHash +
               block.Difficulty))
    hashed := hash.Sum(nil)
    return hex.EncodeToString(hashed)
}

// a function to perform proof of work calculation and return a hash string
func (bc *Blockchain) ProofOfWorkCalc(proof int, previous_proof int, Timestamp int64) string {
    // calculate the proof of work function
    var hash_PoW = sha256.New()
    result := (proof * proof) - (previous_proof * previous_proof) - int(Timestamp)
    hash_PoW.Write([]byte(strconv.Itoa(result)))
    hashed_PoW := hash_PoW.Sum(nil)
    result_hash := hex.EncodeToString(hashed_PoW)
    return result_hash
}

// The core mining function, tries random numbers until finding a golden hash
func (bc *Blockchain) ProofOfWork() (int, int64) {
    rand.Seed(time.Now().UnixNano())
    var r int
    var Timestamp int64
    r = rand.Intn(2147483647)
    for true {
	Timestamp = time.Now().Unix()
        previous_proof := bc.Chain[len(bc.Chain) - 1].Proof
	result_hash := bc.ProofOfWorkCalc(r, previous_proof, Timestamp)

        if strings.Compare(result_hash, bc.Chain[len(bc.Chain) - 1].Difficulty) < 1 {
            break
        }
        r++
    }
    return r, Timestamp
}

// A function to use channels to send the blockchain height to the node package
func (bc *Blockchain) SendHeight() {
    for true {
        i := <-bc.HeightChannel
        if i == 0 {
            bc.HeightChannel <-len(bc.Chain)
        }
    }
}

// A function to use channels to send a block to the node package
func (bc *Blockchain) SendBlocks() {
    for true {
        i := <-bc.BlockIndexChannel
        if i < len(bc.Chain) && i >= 0 {
            bc.GetBlockChannel <-bc.Chain[i]
        } else {
            // make an "error" block
            respBlock := Block {
                Index: -1,
            }
            bc.GetBlockChannel <-respBlock
        }
    }
}

// A function to receive a new block from the node package
func (bc *Blockchain) AddRemoteBlocks() {
    for true {
        // listen for a block from the node goroutine
        newBlock := <-bc.AddBlockChannel
        bc.BlockMutex.Lock()
        bc.Chain = append(bc.Chain, newBlock)
        bc.BlockMutex.Unlock()
        fmt.Println("Another miner found block " + strconv.Itoa(len(bc.Chain)))
        if !bc.ValidateChain() {
            // the new block is invalid, delete it
            bc.BlockMutex.Lock()
            bc.Chain = bc.Chain[:len(bc.Chain) - 1]
            bc.BlockMutex.Unlock()
            // let the node package know that the block was rejected
            bc.BlockValidateChannel <- false
        } else {
            bc.BlockValidateChannel <- true
        }
    }
}

//add function to validate blockchain
func (bc *Blockchain) ValidateChain() bool {
    for i := 1; i <= len(bc.Chain); i++ {
		//current block
        block := bc.Chain[len(bc.Chain) - 1]
		//previous block
	prev_block :=  bc.Chain[len(bc.Chain) - 2]
	proof_hash := bc.ProofOfWorkCalc(block.Proof, prev_block.Proof, block.Timestamp)
	//verify index
        if block.Index != prev_block.Index + 1 {
            fmt.Println("the new block had the wrong index")
            fmt.Println(block)
	    return false
	}
	//verify time stamp
        if block.Timestamp < prev_block.Timestamp {
            fmt.Println("the new block had a bad timestamp")
            fmt.Println(block)
	    return false
	}
	//verify proof
	if strings.Compare(proof_hash, prev_block.Difficulty) != -1 {
            fmt.Println("the new block did not reach the difficulty target")
            fmt.Println(block)
	    return false
        }
	if bc.HashBlock(prev_block) != block.PreviousHash {
            fmt.Println("the new block had a bad previous hash field")
            fmt.Println(block)
	    return false
	}
    }
    return true
}

//Write json to drive
func (bc *Blockchain) WriteChain() {
	jsonChain, err := json.Marshal(bc.Chain)
	if err != nil{
		fmt.Println(err.Error())
		return
	}
	err = ioutil.WriteFile(JSONCHAIN, jsonChain, 0644)
	if err != nil{
		fmt.Println(err.Error())
		return
	}
}


//Read json from drive
func (bc *Blockchain) ReadChain() bool {
	diskChainList := []Block{}

	chainData, err := ioutil.ReadFile(JSONCHAIN)
	if err != nil {
		return false
	}
	err = json.Unmarshal(chainData, &diskChainList)
	if err != nil{
		return false
	}

	bc.Chain = diskChainList
	return true
}
