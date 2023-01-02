package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"
	service "service/interface"
	"google.golang.org/grpc"
)



type RMServer struct {
	service.UnimplementedServiceServer
	id              int32
	peers           map[int32]service.ServiceClient
	isPrimary       bool
	ctx             context.Context
	value      		int32
	valueMap		map[int32]int32
	time            time.Time
	primary         service.ServiceClient
	lastUpdateNodeId int32
	lockChannel 	chan bool
}

func main() {
	portInput, _ := strconv.ParseInt(os.Args[1], 10, 32) //Takes arguments 0, 1 and 2, see comment X
	primary, _ := strconv.ParseBool(os.Args[2])
	ownPort := int32(portInput) + 5000

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rmServer := &RMServer{
		id:              ownPort,
		peers:           make(map[int32]service.ServiceClient),
		ctx:             ctx,
		isPrimary:       primary,
		valueMap: 		 make(map[int32]int32),
		value:      	 int32(0),
		time:            time.Now().Local().Add(time.Second * time.Duration(100)),
		primary:         nil,
		lastUpdateNodeId: int32(0),
		lockChannel: 	 make(chan bool, 1),
	}

	//Unlocking
	rmServer.lockChannel <- true


	fmt.Println("rmServer.isPrimary", rmServer.isPrimary)
	fmt.Println("rmServer.Primary", rmServer.primary)

	//log to file instead of console
	f := setLogRMServer()
	defer f.Close()

	//Primary needs to listen so that replica managers can ask if it's alive
	//Replica managers need to listen for incoming data to be replicated
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", ownPort))
	if err != nil {
		fmt.Printf("Failed to listen on port: %v", err)
		log.Fatalf("Failed to listen on port: %v", err)
	}
	grpcServer := grpc.NewServer()
	service.RegisterServiceServer(grpcServer, rmServer)
	go func() {
		if err := grpcServer.Serve(list); err != nil {
			fmt.Printf("failed to server %v", err)
			log.Fatalf("failed to server %v", err)
		}
	}()

	for i := 0; i < 3; i++ {
		port := int32(5000) + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		log.Printf("RMServer %v: Trying to dial: %v\n", rmServer.id, port)
		fmt.Printf("RMServer %v: Trying to dial: %v\n", rmServer.id, port)
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock()) //This is going to wait until it receives the connection
		if err != nil {
			fmt.Printf("Could not connect: %s", err)
			log.Fatalf("Could not connect: %s", err)
		}
		defer conn.Close()
		c := service.NewServiceClient(conn)
		rmServer.peers[port] = c
		if port == 5000 {
			rmServer.primary = c
		}

		fmt.Println("rmServer.Primary", rmServer.primary)
	}

	// If server is primary, dial to all other replica managers
	if !rmServer.isPrimary {
		go func() {
			for {
				time.Sleep(2 * time.Second)
				heartbeatMsg := &service.HeartbeatRequest{Message: "alive?"}
				_, err := rmServer.primary.GetHeartBeat(rmServer.ctx, heartbeatMsg)
				if err != nil {
					fmt.Printf("RMServer %v: Something went wrong while sending heartbeat", rmServer.id)
					fmt.Printf("RMServer %v: Error: %s, \n Deleting node. ", rmServer.id, err)
					log.Printf("RMServer %v: Something went wrong while sending heartbeat", rmServer.id)
					log.Printf("RMServer %v: Error: %s, \n Deleting node. ", rmServer.id, err)
					delete(rmServer.peers, 5000)
					rmServer.ElectLeader()
				}
				
				//fmt.Printf("We got a heart beat from %s", response)

				if rmServer.isPrimary {
					break
				}
			}
		}()

		for {}
	}
	for {

	}

}

func (RM *RMServer) ElectLeader() {
	fmt.Printf("RMServer %v: Leader election started with Bully Algorithm", RM.id)
	log.Printf("RMServer %v: Leader election started with Bully Algorithm", RM.id)
	var min int32
	min = RM.id
	for id := range RM.peers {
		if min > id {
			min = id
		}
	}

	if RM.id == min {
		RM.isPrimary = true
	} else {
		RM.primary = RM.peers[min]
	}
	fmt.Printf("RMServer %v: New Primary Replica has port %v ", RM.id, min)
	log.Printf("RMServer %v: New Primary Replica has port %v ", RM.id, min)
}

func (RM *RMServer) GetHeartBeat(ctx context.Context, Heartbeat *service.HeartbeatRequest) (*service.HeartbeatAck, error) {
	return &service.HeartbeatAck{Port: fmt.Sprint(RM.id)}, nil
}

func (RM *RMServer) Update(ctx context.Context, uptRequest *service.UpdateRequest) (*service.UpdateResponse, error) {
	<- RM.lockChannel
	fmt.Println("inside update")
	RM.value = uptRequest.Value
	if !RM.isPrimary {
		return &service.UpdateResponse{Outcome: true}, nil
	}
	validUpdates := 0
	for backupID, backup := range RM.peers {
		_, err := backup.Update(ctx, uptRequest)
		if err != nil {
			log.Printf("RMServer: Something went wrong when updating to %v", backupID)
 			log.Printf("RMServer: Exception, Replica Manager on port %v died", backupID)
			delete(RM.peers, backupID)
		} else {
			validUpdates++
		}
	}

	//var quorum float32

	fmt.Println("validUpdates/len(RM.peers)", float32(validUpdates/len(RM.peers)))
	//Checking for quorum
	if float32((validUpdates/len(RM.peers))) > float32(0.5) {
		updateResonse := &service.UpdateResponse{
			Outcome: true,
		}
		return updateResonse, nil
	}

	RM.lockChannel <- true

	return nil, nil
}

func (RM *RMServer) Retrieve(ctx context.Context, retrieveReq *service.RetrieveRequest) (*service.RetrieveResponse, error) {
	fmt.Println("inside retrieve")
	//value := RM.valueMap[retrieveReq.Id]
	//fmt.Printf("RMServer: Outcome inside Result: %v", value)
	return &service.RetrieveResponse{
		Outcome: RM.value,
	}, nil
}

// sets the logger to use a log.txt file instead of the console
func setLogRMServer() *os.File {
	f, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)
	return f
}