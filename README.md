# Project 4: Bitcoin
**Shiyin Wang** (2020 Spring Tsinghua Operating Systems Group Project)

[GitHub](https://github.com/shiyinw/bitcoin-from-scratch/) https://github.com/shiyinw/bitcoin-from-scratch/

Built on top of Project 3, the goal of Project 4 is to achieve high-availability / crash-tolerance by replicating blocks onto multiple servers.

**Overview**

In the lecture, we have discussed several protocols to achieving consensus during replication. In this project, we ask you to adopt the "bitcoin" style of consensus, to achieve distributed consensus and tamper-proof validity. In this assignment, we implement a simplified blockchain system and see how the concepts like Proof-of-Work.

We reused some of your key-value database codebase from Project 3. The client interface is similar to the gRPC interface in Project 3. Our extra goal is reach consensus on the content of each block, even if a small percentage of servers are *malicious*, i.e. they do not follow the protocols. To achieve the goal, we need to support server-to-server replications. Thus, we need to introduce an additional set of gRPC interfaces for communication among these servers.

In a nutshell, the system works the following way:

1. The clients contact any running server to submit a transaction.

2. The server broadcasts the (tentative) transactions to the entire network, hoping that most of the

   servers learns about it.

3. The servers run a consensus protocol (e.g. proof-of-work) to determine a server as the leader.

4. The elected leader will assemble the transactions into a valid (valid means meeting integrity

   constraints defined in the previous project) ``block’’, in an order of her choice, and broadcast the

   block to all servers.

5. Other servers verify the following before committing a block to their view of the block chain:

   1. The leader is really a leader elected, by verifying the validity of proof-of-work;
   2. The block satisfies all the integrity constraints.



### 1. (30%, 50 lines) Basic database operations.



### 2. (20%, 20 lines) Reject invalid blocks.



### 3. (15%, 20 lines) Integrity constrains.
Similar with the project 3, whenever we want to reduce the amount of money from a user, we need to check his/her balance first. The implementation is simple.
```go
func (s *server) Transfer(ctx context.Context, in *pb.TransferRequest) (*pb.BooleanResponse, error) {
  ......
	if data[in.FromID]>=in.Value && in.FromID!=in.ToID{
		loglen++
		data[in.FromID] -= in.Value
		data[in.ToID] += in.Value
    data[file.MinerID] += in.MiningFee
		return &pb.BooleanResponse{Success: true}, nil
	}else{
		return &pb.BooleanResponse{Success: false}, errors.New("Transaction "+string(in.Value)+" failed with: insufficient balance")
	}
}
```

We also need to avoid contradictory transactions. We have observed that the problem happens on the senders' side. Therefore, we assign a lock to each user to constrain that **only one thread can decrease the balance of a certain user**. And we use the `defer` syntax to unlock. Here `mutex` is an instance of `github.com/EagleChen/mapmutex`.

```go
got := false
for true {
  if got = mutex.TryLock(in.UserID); got {
    break
  }
}
if got {
  // code here
  ......
  return &pb.BooleanResponse{Success: true}, nil
}
return &pb.BooleanResponse{Success: false}, nilA
```

### 4. (20%, 30 lines) Maintain the structure of blockchain.



### 5. (10%, 2 lines) The compile script to(compile.sh), and a startup script(start.sh).

We used two external packages. You may need to install them in advance.

```
go get github.com/EagleChen/mapmutex
go get github.com/google/uuid
```

You can simply use the `compile.sh` and `start.sh`, which are provided in the skeleton, to run our blockchain ledger.

```bash
./compile.sh
./start.sh
```

Since in this project, we don't initialize the client by `PUT` command, so if you should make sure that the ports are unoccupied. Otherwise our model can't bind to the port and what you actually interact with may be other model.

```
lsof -n -i4TCP:50051 | grep LISTEN | awk '{ print $2 }' | xargs kill
```

### 6. (20%, unknown lines) Test cases.

We implemented several tests in `./test/` directory, you can run them together by:

```bash
./test/run.sh
```

Our test covers several aspects of this system.

| Script    | \# Lines | Features                                                     |
| --------- | -------- | ------------------------------------------------------------ |
| test_0.go |          | server start and shutdown, client initialization with 1000 balance |
| test_1.go |          | broadcase transactions to other servers according to `config.json` |
| test_2.go |          | the transfers in a server is acknowledged by other servers   |
|           |          |                                                              |
|           |          |                                                              |

### Bonus. (20%, unknown lines) Less-than-linear response time when there are over 1 million blocks.