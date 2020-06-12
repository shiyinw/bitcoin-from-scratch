package main

// Test 0: start and shutdown

import (
	pb "blockchaindb_go/protobuf/go"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io/ioutil"
	"log"
	"math/rand"
	"os/exec"
	"sync"
	"time"
)

var address = []string{"127.0.0.1:50053", "127.0.0.1:50054"}
var config = `
{
	"1":{
		"ip":"127.0.0.1",
		"port":"50053",
		"dataDir":"tmp/server03/"
	},
	"2":{
		"ip":"127.0.0.1",
		"port":"50054",
		"dataDir":"tmp/server04/"
	},
	"nservers":2
}
`

var err error
var N int = 2

func id(i int) string {
	return fmt.Sprintf("T1U%05d", i)
}

func UUID128bit() string {
	// Returns a 128bit hex string, RFC4122-compliant UUIDv4
	u:=make([]byte,16)
	_,_=rand.Read(u)
	// this make sure that the 13th character is "4"
	u[6] = (u[6] | 0x40) & 0x4F
	// this make sure that the 17th is "8", "9", "a", or "b"
	u[8] = (u[8] | 0x80) & 0xBF
	return fmt.Sprintf("%x",u)
}


func main() {

	// JSON config
	// fmt.Println("Writing config JSON...", config)
	ioutil.WriteFile("./config.json", []byte(config), 0644)


	// Step 1: start servers
	cmd1 := exec.Command("./start.sh", "--id=1")
	err = cmd1.Start()
	defer cmd1.Process.Kill()
	if err != nil{
		log.Fatalf("./start.sh returned error: %v", err)
	}
	cmd2 := exec.Command("./start.sh", "--id=2")
	err = cmd2.Start()
	defer cmd2.Process.Kill()
	if err != nil{
		log.Fatalf("./start.sh returned error: %v", err)
	}


	// wait for a while
	time.Sleep(time.Second * 2)

	// Set up a connection to the server.
	conn1, e1 := grpc.Dial(address[0], grpc.WithInsecure())
	defer conn1.Close()
	conn2, e2 := grpc.Dial(address[1], grpc.WithInsecure())
	defer conn2.Close()
	if e1 != nil || e2 != nil{
		log.Fatalf("Failed to establish connection to server (1s after startup)")
	}


	// Step 2: Initial a new client (1000)
	client1 := pb.NewBlockChainMinerClient(conn1)
	client2 := pb.NewBlockChainMinerClient(conn2)
	ctx := context.Background()

	// Test Transfer
	var wg sync.WaitGroup
	fmt.Println("Part 1: Make 100 transfer from i to i+1 in server 1")

	var N int = 10
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			if r, err := client1.Transfer(context.Background(), &pb.Transaction{
				Type:pb.Transaction_TRANSFER,
				UUID:UUID128bit(),
				FromID: id(i), ToID: id(i+1), Value: 100, MiningFee: 3}); err != nil {
				log.Printf("TRANSFER(%s -> %s) Error: %v", id(i), id(i+1), err)
			} else {
				log.Printf("TRANSFER(%s -> %s) Return: %s", id(i), id(i+1), r)
			}
			//time.Sleep(time.Millisecond * 100)
			wg.Done()
		}(i)
		wg.Wait() // Otherwise, "transport is closing"
	}


	// Check Server 1
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			uid := id(i)
			r, err := client1.Get(ctx, &pb.GetRequest{UserID: uid})
			if err!=nil{
				log.Fatalf("client1.Get: %v", err)
			}
			if (i>0 && i<N && r.Value != 1000)||(i==0 && r.Value!=900)||(i==N && r.Value!=1100){
				log.Fatalf("client1.Get false return value (user %s = %d)", id(i), r.Value)
			}
			wg.Done()
		}(i)
		wg.Wait() // Otherwise, "transport is closing"
	}


	// Check Server 2
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			uid := id(i)
			r, err := client2.Get(ctx, &pb.GetRequest{UserID: uid})
			if err!=nil{
				log.Fatalf("Client.Get: %v", err)
			}
			if (i>0 && i<N && r.Value != 1000)||(i==0 && r.Value!=900)||(i==N && r.Value!=1100){
				log.Fatalf("Client.Get false return value (user %d = %d)", i, r.Value)
			}
			wg.Done()
		}(i)
		wg.Wait()
	}


	log.Println("Part 1 Success.")
}
