package main

import (
	service "service/interface"
	grpc "google.golang.org/grpc"
	"context"
	"os"
	"log"
	"net"
	"fmt"
	"strconv"
)

type FEServer struct {
	service.UnimplementedServiceServer        
	port                               string
	primaryServer                      service.ServiceClient
	ctx                                context.Context
}

var serverToDial int

func main() {
	f := setLogFEServer()
	defer f.Close()

	/*
	Set listening port to listen to client messages; 
	- takes first argument after ´go run feserver.go´
	- accepted values: 4000 or 4001
	*/
	port := os.Args[1] 
	address := ":" + port
	list, err := net.Listen("tcp", address)

	if err != nil {
		log.Printf("FEServer %s: Server on port %s: Failed to listen on port %s: %v", port, port, address, err) //If it fails to listen on the port, run launchServer method again with the next value/port in ports array
		return
	}
	grpcServer := grpc.NewServer()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := &FEServer{
		port:          os.Args[1],
		primaryServer: nil,
		ctx:           ctx,
	}

	service.RegisterServiceServer(grpcServer, server) 
	fmt.Printf("FEServer %s: Server on port %s: Listening at %v\n", server.port, port, list.Addr())
	log.Printf("FEServer %s: Server on port %s: Listening at %v\n", server.port, port, list.Addr())

	
	go func() {
		fmt.Printf("FEServer _attempting_ to listen on port %s \n", server.port)
		log.Printf("FEServer _attempting_ to listen on port %s \n", server.port)
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("Failed to serve on port %s: %v", server.port, err)
		}
		fmt.Printf("FEServer %s successfully listening for requests.", server.port)
		log.Printf("FEServer %s successfully listening for requests.", server.port)
	}()

	serverToDial = 5000
	conn := server.DialToPR(serverToDial)

	defer conn.Close()

	for{}
}

func (FE *FEServer) DialToPR(serverToDial int) *grpc.ClientConn {
	//Dialing to the primary replica manager
	portToDial := ":" + strconv.Itoa(serverToDial)
	connection, err := grpc.Dial(portToDial, grpc.WithInsecure())
	if err != nil {
		fmt.Printf("Unable to connect: %v", err)
		log.Fatalf("Unable to connect: %v", err)
	}
	fmt.Printf("FEServer %s: Connection established with Primary Replica.", FE.port)
	log.Printf("FEServer %s: Connection established with Primary Replica.", FE.port)
	primServer := service.NewServiceClient(connection)
	FE.primaryServer = primServer
	return connection
}

func (FE *FEServer) Redial(functionType string, updateMsg *service.UpdateRequest, key int32) (*service.RetrieveResponse, *service.UpdateResponse) {

	portNumber := int64(serverToDial) + int64(1)

	fmt.Printf("FEServer %s: Dialing to new PrimaryReplica on port ", FE.port, portNumber)
	log.Printf("FEServer %s: Dialing to new PrimaryReplica on port ", FE.port, portNumber)
	FE.DialToPR(int(portNumber))
	if functionType == "result" {
		getResult := &service.RetrieveRequest{
			Id: key,
		}
		outcome, _ := FE.Retrieve(FE.ctx, getResult)
		return outcome, nil
	} else { 
		outcome, _ := FE.Update(FE.ctx, updateMsg)
		return nil, outcome
	}
}

func (FE *FEServer) Update(ctx context.Context, updateMsg *service.UpdateRequest) (*service.UpdateResponse, error) {
	fmt.Println("inside update")
	outcome, err := FE.primaryServer.Update(ctx, updateMsg)
	if err != nil {
		log.Printf("FEServer %s: Error: %s", FE.port, err)
		_, redialOutcome := FE.Redial("update", updateMsg, -1)
		return redialOutcome, nil
	}
	return outcome, nil
}

func (FE *FEServer) Retrieve(ctx context.Context, retrieveReq *service.RetrieveRequest) (*service.RetrieveResponse, error) {
	fmt.Println("inside retrieve")
	outcome, err := FE.primaryServer.Retrieve(ctx, retrieveReq)
	if err != nil {
		log.Printf("FEServer %s: Error %s", FE.port, err)

		//if we get an error we need to Dial to another port
		result, _ := FE.Redial("result", nil, 0)
		return result, nil
	}

	return outcome, nil
}


//Logs to file.txt
func setLogFEServer() *os.File {
	f, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)
	return f
}
