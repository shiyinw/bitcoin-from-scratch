package main

// Test 0: start and shutdown

import (
	"io/ioutil"
	"log"
	"os/exec"
	"time"
	"fmt"
	pb "blockchaindb_go/protobuf/go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var address = []string{"127.0.0.1:50051", "127.0.0.1:50052"}
var config = `
{
	"1":{
		"ip":"127.0.0.1",
		"port":"50051",
		"dataDir":"tmp/server01/"
	},
	"2":{
		"ip":"127.0.0.1",
		"port":"50052",
		"dataDir":"tmp/server02/"
	},
	"nservers":2
}
`

var cmd = [](*exec.Cmd){}
var conn = [](*grpc.ClientConn){}
var err error
var N int = 2
var new_cmd *exec.Cmd
var new_conn *grpc.ClientConn

func main() {

	// JSON config
	// fmt.Println("Writing config JSON...", config)
	ioutil.WriteFile("./config.json", []byte(config), 0644)


	// Step 1: start servers
	cmd1 := exec.Command("./start.sh", "--id=1")
	err = cmd1.Start()
	if err != nil{
		log.Fatalf("./start.sh returned error: %v", err)
	}

	// wait for a while
	time.Sleep(time.Second * 2)

	// Set up a connection to the server.
	conn1, e1 := grpc.Dial(address[0], grpc.WithInsecure())
	if e1 != nil{
		log.Fatalf("Failed to establish connection to server (1s after startup)")
	}

	// Step 2: Initial a new client (1000)
	c := pb.NewBlockChainMinerClient(conn1)
	ctx := context.Background()

	r, err := c.Get(ctx, &pb.GetRequest{UserID: "NONEXIST"})
	if err != nil {
		log.Fatalf("Error requesting server (1s after startup)")
	}
	if r.Value != 1000 {
		log.Fatalf("Value!=1000. Not a clean start?")
	}
	fmt.Println("Check the initial value is 1000")

	// Step 3: kill process
	err = cmd1.Process.Kill()
	if err != nil {
		log.Fatalf("Failed to kill process.")
	}
	err = conn1.Close()
	if err != nil {
		log.Fatalf("Failed to kill process.")
	}

	time.Sleep(time.Millisecond * 1)
	r, err = c.Get(ctx, &pb.GetRequest{UserID: "NONEXIST"})
	if err == nil {
		log.Fatalf("Failed to kill process; server still responding.")
		log.Print(err)
	}

	log.Println("Test 0 Passed.")
}
