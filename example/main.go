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
var fileidx int64 = 1 // store the ledger in [fileidx].json
var dataDir string // store the blocks
var server_list []string
var neighbors_client = make(map[string]pb.BlockChainMinerClient)
var neighbors_conn = make(map[string]grpc.ClientConn)
var address string

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

// store the local data as a tree
type TreeNode struct {
	Prev  *TreeNode
	Depth int //fileidx
	Cache map[string]int32 // data [id]->balance
	FileIO fileio // Block content
	Hash string // 64-digit hash
	Next []*TreeNode // bidirectional
}


type fileio struct{
	BlockID int64
	PrevHash string `default:"0000000000000000000000000000000000000000000000000000000000000000"`
	Transactions []transactionio
	MinerID string
	Nonce string `default:"00000000"`
} //redefine for the pb.Block has nuisance attributes

var prevHash string = "0000000000000000000000000000000000000000000000000000000000000000"
var nonce string = "00000000"
var file fileio = fileio{fileidx, prevHash, []transactionio{}, "Server",nonce}
var mineflag = false

func Mine(filecontent fileio, start int)(bool, fileio){
	for mineflag && len(filecontent.Transactions)>0{
		filecontent.Nonce = fmt.Sprintf("%08d", start)
		curStr := fmt.Sprintf("%v", filecontent)
		if hash.CheckHash(curStr) {
			return true, filecontent
		}
		start++
	}
	return false, filecontent
}

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
			data[file.MinerID] += in.MiningFee
			// Json I/O
			entry := transactionio{"TRANSFER", in.FromID, in.ToID, in.Value, in.MiningFee, in.UUID}
			file.Transactions = append(file.Transactions, entry)
			data, err1 := json.Marshal(file)
			if err1 != nil {
				return &pb.BooleanResponse{Success: false}, err1
			}
			err2 := ioutil.WriteFile(dataDir+strconv.FormatInt(fileidx, 10)+".json", data, 0644)
			if err2 != nil {
				return &pb.BooleanResponse{Success: false}, err2
			}
			if loglen % maxBlockSize == 0 {
				loglen = 0
				fileidx++
				file.BlockID = fileidx
				file.Transactions = []transactionio{}
			}
			if in.Type == 5{  // TRANSFER
				in.Type = 4
				count, err := s.PushTransaction(ctx, in)
				if err==nil && count.Number>=int32(1){
					return &pb.BooleanResponse{Success: true}, nil
				}else{
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
		log.Print(addr)
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

	file = fileio{fileidx, prevHash, []transactionio{}, MinerID,nonce}
	

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
		log.Print(server_list)
		dat = dat[IDstr].(map[string]interface{}) // should be dat[myNum] in the future
		return fmt.Sprintf("%s:%s", dat["ip"], dat["port"]), fmt.Sprintf("%s",dat["dataDir"])
	}()
	//Different from Project 3, when a server crashes, it loses all data on disk. The server should recover its blocks by replicating from another server.
	os.RemoveAll(dataDir)
	os.Mkdir(dataDir, 0777)
	log.Print(address)
	log.Print(dataDir)

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

	// Start server
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
