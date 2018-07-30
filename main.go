package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
)

// Block declare structure of block
type Block struct {
	Index     int
	Timestamp string
	Message   string
	Hash      string
	PrevHash  string
	Validator string
}

// Blockchain is a list of blocks
var blockchain []Block
var tempBlocks []Block

// Next blocks for validating
var candidateBlocks = make(chan Block)

// annouce winner to all validator
var announcements = make(chan string)

var mutex = &sync.Mutex{}

// validator -> balance
var validators = make(map[string]int)

// calculateHash SHA256 hashing function
func calculateHash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// calculateBlockHash calculates the hash value of the block's infomation
func calculateBlockHash(block Block) string {
	blockInfo := string(block.Index) + block.Timestamp + string(block.Message) + block.PrevHash
	return calculateHash(blockInfo)
}

// generateBlock generate a new block
func generateBlock(prevBlock Block, message string, address string) (Block, error) {
	var newBlock Block

	t := time.Now()

	newBlock.Index = prevBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.Message = message
	newBlock.PrevHash = prevBlock.Hash
	newBlock.Hash = calculateBlockHash(newBlock)
	newBlock.Validator = address

	return newBlock, nil
}

//isBlockValid checks if the block is valid
func isBlockValid(prevBlock, newBlock Block) bool {
	if prevBlock.Index+1 != newBlock.Index ||
		prevBlock.Hash != newBlock.PrevHash ||
		calculateBlockHash(newBlock) != newBlock.Hash {
		return false
	}
	return true
}

//handleConn handle connnections to TCP server
func handleConn(conn net.Conn) {
	defer conn.Close()

	// annouce message to validators
	go func() {
		for {
			msg := <-announcements
			io.WriteString(conn, msg)
		}
	}()

	// validator address
	var address string

	// get balance
	io.WriteString(conn, "Enter token balance: ")
	scanBalance := bufio.NewScanner(conn)
	for scanBalance.Scan() {
		balance, err := strconv.Atoi(scanBalance.Text())
		if err != nil {
			log.Printf("%v not a number: %v", scanBalance.Text(), err)
			return
		}
		t := time.Now()
		address = calculateHash(t.String())
		validators[address] = balance
		fmt.Println(validators)
		break
	}

	io.WriteString(conn, "\nEnter a new message:")
	scanMessage := bufio.NewScanner(conn)

	go func() {
		for {
			for scanMessage.Scan() {
				message := scanMessage.Text()

				mutex.Lock()
				prevBlock := blockchain[len(blockchain)-1]
				mutex.Unlock()

				// generate new block
				newBlock, err := generateBlock(prevBlock, message, address)
				if err != nil {
					log.Printf("err")
					continue
				}
				if isBlockValid(prevBlock, newBlock) {
					candidateBlocks <- newBlock
				}
				io.WriteString(conn, "\nEnter a new message:")
			}
		}
	}()

	for {
		time.Sleep(time.Minute)
		mutex.Lock()
		output, err := json.Marshal(blockchain)
		mutex.Unlock()
		if err != nil {
			log.Fatal(err)
		}
		io.WriteString(conn, string(output)+" BLOCKCHAIN \n")
	}

}

// pick up the winner to validate the new block
// Proof of Stake
func pickValidator() {
	time.Sleep(30 * time.Second)
	mutex.Lock()
	temp := tempBlocks
	mutex.Unlock()

	// create a lottery pool
	lotteryPool := []string{}

	if len(temp) > 0 {

	OUTER:
		for _, block := range temp {
			for _, node := range lotteryPool {
				if block.Validator == node {
					continue OUTER
				}
			}

			mutex.Lock()
			setValidators := validators
			mutex.Unlock()

			k, ok := setValidators[block.Validator]
			if ok {
				for i := 0; i < k; i++ {
					lotteryPool = append(lotteryPool, block.Validator)
				}
			}
		}

		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)

		// randomly pick up the winner from the pool
		lotteryWinner := lotteryPool[r.Intn(len(lotteryPool))]

		for _, block := range temp {
			if block.Validator == lotteryWinner {
				mutex.Lock()
				blockchain = append(blockchain, block)
				mutex.Unlock()
				for _ = range validators {
					announcements <- "\nWinning validator: " + lotteryWinner + "\n"
				}
				break
			}
		}
	}

	mutex.Lock()
	tempBlocks = blockchain
	mutex.Unlock()
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	t := time.Now()

	// create genesis block - the head of blockchain
	genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), "Genesis block", calculateBlockHash(genesisBlock), "", ""}
	spew.Dump(genesisBlock)
	blockchain = append(blockchain, genesisBlock)

	server, err := net.Listen("tcp", ":"+os.Getenv("PORT"))
	if err != nil {
		log.Println("error happened!!!")
		log.Fatal(err)
	}
	defer server.Close()

	go func() {
		for candidate := range candidateBlocks {
			mutex.Lock()
			blockchain = append(blockchain, candidate)
			mutex.Unlock()
		}
	}()

	go func() {
		for {
			pickValidator()
			spew.Dump(blockchain)
		}
	}()

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}

}
