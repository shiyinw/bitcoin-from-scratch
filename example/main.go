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

type fileio struct{
	BlockID int64
	PrevHash string `default:"0000000000000000000000000000000000000000000000000000000000000000"`
	Transactions []transactionio
	MinerID string
	Nonce string `default:"00000000"`
}

var prevHash string = "0000000000000000000000000000000000000000000000000000000000000000"
var nonce string = "0000000000000000000000000000000000000000000000000000000000000000"
var file fileio = fileio{fileidx, prevHash, []transactionio{}, "Server",nonce}



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
		if data[in.FromID] >= in.Value {
			loglen++
			data[in.FromID] -= in.Value
			data[in.ToID] += in.Value
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
func (s *server) PushTransaction(ctx context.Context, in *pb.Transaction) (*pb.Null, error) {
	return &pb.Null{}, nil
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
	address, outputDir := func() (string, string) {
		conf, err := ioutil.ReadFile("config.json")
		if err != nil {
			panic(err)
		}
		var dat map[string]interface{}
		err = json.Unmarshal(conf, &dat)
		if err != nil {
			panic(err)
		}
		dat = dat[IDstr].(map[string]interface{}) // should be dat[myNum] in the future
		return fmt.Sprintf("%s:%s", dat["ip"], dat["port"]), fmt.Sprintf("%s",dat["dataDir"])
	}()
	dataDir = outputDir
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
