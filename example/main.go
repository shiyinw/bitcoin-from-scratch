package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"sync"

	"blockchaindb_go/hash"
	pb "blockchaindb_go/protobuf/go"

	"github.com/EagleChen/mapmutex"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)


const maxBlockSize = 50
type Dictionary map[string]interface{}

var data = make(map[string]int32)
var loglen int32
var fileidx int32 = 1 // store the ledger in [fileidx].json
var dataDir string // store the blocks
var server_list []string
var neighbors_client = make(map[string]pb.BlockChainMinerClient)
var neighbors_conn = make(map[string]grpc.ClientConn)
var address string

var blockHashTable = make(map[string]pb.Block) //store the hash table

// no two contradictory transactions
var mutex = mapmutex.NewMapMutex()

// json I/O
type transactionio struct{
	Type string
	FromID string
	ToID string
	Value int32
	MiningFee int32
	UUID string
}

var prevHash string = "0000000000000000000000000000000000000000000000000000000000000000"
var nonce string = "00000000"

var m = &sync.Mutex{}
var c = sync.NewCond(m)
var curHash string
var chanBlock = make(chan *pb.Block)
var curBlock pb.Block = pb.Block{BlockID: fileidx, PrevHash: prevHash, Transactions: []*pb.Transaction{}, MinerID: "", Nonce: nonce}




type server struct{}
// Database Interface 
func (s *server) Get(ctx context.Context, in *pb.GetRequest) (*pb.GetResponse, error) {
	// A new account will have balance 1000 instead of 0.
	var init bool
	if _, init = data[in.UserID]; !init{
		data[in.UserID] = 1000
	}
	return &pb.GetResponse{Value: data[in.UserID]}, nil
}

var updateBlock bool = true

func Mine()() {
	log.Print("Start Mining")
	defer log.Print("Stop Mining")
	var start int = 0
	var block pb.Block
	var data []byte
	for{
		if updateBlock{
			block = curBlock
			updateBlock = false
			start = 0
			data, _ = json.Marshal(block)
		}
		if len(block.Transactions)<=0{
			continue
		}
		d, _ := json.Marshal(fmt.Sprintf("%08d", start))
		// can't assign to slice, so we have to enumerate one-by-one
		data[len(data)-10] = d[1]
		data[len(data)-9] = d[2]
		data[len(data)-8] = d[3]
		data[len(data)-7] = d[4]
		data[len(data)-6] = d[5]
		data[len(data)-5] = d[6]
		data[len(data)-4] = d[7]
		data[len(data)-3] = d[8]
		//log.Print(string(data))
		//log.Print(data)
		//time.Sleep(time.Second)
		hashStr := hash.GetHash(data)
		//log.Print(hashStr)
		if hash.CheckHash(hashStr){
			err2 := ioutil.WriteFile(dataDir+strconv.FormatInt(int64(fileidx), 10)+".json", data, 0644)
			if err2 != nil{
				log.Fatalf("%v", err2)
			}
			loglen = 0
			fileidx++
			curBlock.BlockID = fileidx
			curBlock.PrevHash = hashStr
			curBlock.Transactions = []*pb.Transaction{}
			updateBlock = true
		}
		start++
	}
}

func (s *server) Transfer(ctx context.Context, in *pb.Transaction) (*pb.BooleanResponse, error) {
	// A new account will have balance 1000 instead of 0.
	var init bool
	if _, init = data[in.FromID]; !init{
		data[in.FromID] = 1000
	}
	if _, init = data[in.ToID]; !init{
		data[in.ToID] = 1000
	}

	got := false
	for true {
		if got = mutex.TryLock(in.FromID); got {
			break
		}
	}
	if got {
		defer mutex.Unlock(in.FromID)
		// Integrity constrain
		var userid = in.UUID
		if data[in.FromID] >= in.Value && in.FromID!=in.ToID{
			loglen++
			data[in.FromID] -= in.Value
			data[in.ToID] += in.Value
			data[curBlock.MinerID] += in.MiningFee
			// Json I/O

			log.Printf("[%s] Transaction  %s -> %s (%d)", address, in.FromID, in.ToID, in.Value)
			if in.Type == 5{  // TRANSFER
				in.Type = 4
				count, err := s.PushTransaction(ctx, in)
				if err!=nil{
					return &pb.BooleanResponse{Success: false}, err
				}
				if count.Number>=int32(1){
					entry := pb.Transaction{Type: 5, FromID: in.FromID, ToID: in.ToID, Value: in.Value, MiningFee: in.MiningFee, UUID: in.UUID}
					if loglen < maxBlockSize{
						curBlock.Transactions = append(curBlock.Transactions, &entry)
						updateBlock = true
					}
					return &pb.BooleanResponse{Success: true}, nil
				}else{
					log.Printf("Transaction unrecognized by other users")
					return &pb.BooleanResponse{Success: false}, nil
				}
			}
			return &pb.BooleanResponse{Success: true}, nil
		} else {
			log.Printf("Transaction %d failed with: %s", in.Value, userid)
			return &pb.BooleanResponse{Success: false}, nil
		}
	}
	return &pb.BooleanResponse{Success: false}, nil
}
func (s *server) Verify(ctx context.Context, in *pb.Transaction) (*pb.VerifyResponse, error) {
	return &pb.VerifyResponse{Result: pb.VerifyResponse_FAILED, BlockHash:"?"}, nil
}

func (s *server) GetHeight(ctx context.Context, in *pb.Null) (*pb.GetHeightResponse, error) {
	return &pb.GetHeightResponse{Height: 1, LeafHash: "?"}, nil
}
func (s *server) GetBlock(ctx context.Context, in *pb.GetBlockRequest) (*pb.JsonBlockString, error) {
	return &pb.JsonBlockString{Json: "{}"}, nil
}
func (s *server) PushBlock(ctx context.Context, in *pb.JsonBlockString) (*pb.Null, error) {
	return &pb.Null{}, nil
}
func (s *server) PushTransaction(ctx context.Context, in *pb.Transaction) (*pb.IntegerResponse, error) {
	var count int32
	for _, addr := range server_list{
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		defer conn.Close()
		if err == nil {
			client := pb.NewBlockChainMinerClient(conn)
			client.Transfer(ctx, in)
			log.Printf("PushTransaction %s -> %s", address, addr)
			count++
		}
	}
	return &pb.IntegerResponse{Number:count}, nil
}

var id=flag.Int("id",1,"Server's ID, 1<=ID<=NServers")

// Main function, RPC server initialization
func main() {
	flag.Parse()
	IDstr:=fmt.Sprintf("%d",*id)

	MinerID := fmt.Sprintf("Server%02d",*id) //MinerID=
	_=hash.GetHashString

	//file = fileio{fileidx, prevHash, []transactionio{}, MinerID,nonce}
	curBlock = pb.Block{BlockID: fileidx, PrevHash: prevHash, Transactions: []*pb.Transaction{}, MinerID: MinerID, Nonce: nonce}

	// Read config
	address, dataDir = func() (string, string) {
		conf, err := ioutil.ReadFile("config.json")
		if err != nil {
			panic(err)
		}
		var dat map[string]interface{}
		err = json.Unmarshal(conf, &dat)
		if err != nil {
			panic(err)
		}
		for i:=1;i<=int(dat["nservers"].(float64));i++{
			if i!=*id{
				curItem := dat[fmt.Sprintf("%d", i)].(map[string]interface{})
				curAddr := fmt.Sprintf("%s:%s", curItem["ip"], curItem["port"])
				server_list = append(server_list, curAddr)
			}
		}
		dat = dat[IDstr].(map[string]interface{}) // should be dat[myNum] in the future
		return fmt.Sprintf("%s:%s", dat["ip"], dat["port"]), fmt.Sprintf("%s",dat["dataDir"])
	}()
	//Different from Project 3, when a server crashes, it loses all data on disk. The server should recover its blocks by replicating from another server.
	os.RemoveAll(dataDir)
	os.Mkdir(dataDir, 0777)
	log.Printf("Initialize %s %s Neighbors:%v", address, dataDir, server_list)

	// Bind to port
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("Listening: %s ...", address)

	// Create gRPC server
	s := grpc.NewServer()
	pb.RegisterBlockChainMinerServer(s, &server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)

	go Mine()

	// Start server
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

}
