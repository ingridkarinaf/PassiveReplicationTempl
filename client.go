package main

import (
	service "service/interface"
	grpc "google.golang.org/grpc"
	"fmt"
	"strings"
	"strconv"
	"log"
	"os"
	"bufio"
	"context"
	"reflect"
)

/* 
Responsible for:
	1. Making connection to FE, has to redial if they lose connection
	2. 
Limitations:
	1. Can only dial to pre-determined front-ends or by incrementing 
	(then there is no guarantee that there is an FE with that port number)
	2. Assumes a failed request is due to a crashed server, redials immediately
*/

var server service.ServiceClient
var connection *grpc.ClientConn 

func main() {

	//Creating log file
	f := setLogClient()
	defer f.Close()

	FEport := ":" + os.Args[1] //dial 4000 or 4001 (available ports on the FEServers)
	conn, err := grpc.Dial(FEport, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Unable to connect: %v", err)
	}
	connection = conn

	server = service.NewServiceClient(connection) //creates a connection with an FE server
	defer connection.Close()

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		
		for {
			println("Enter 'update' to update value, 'get' to get value (without quotation marks)")
			scanner.Scan()
			textChoice := scanner.Text()
			if (textChoice == "update") {
				println("Enter value (integers only!)")
				scanner.Scan()
				text := scanner.Text()
				inputArray := strings.Fields(text)
				
				// key, err := strconv.Atoi(inputArray[0])
				// if err != nil {
				// 	log.Printf("Couldn't convert key to int: ", err)
				// 	continue
				// }

				value, err := strconv.Atoi(inputArray[0])
				if err != nil {
					log.Printf("Couldn't convert value to int: ", err)
					continue
				}
	
				serviceUpdate := &service.UpdateRequest{
					// Key:   int32(key),
					Value: int32(value),
				}

				result := Update(serviceUpdate)
				if result.Outcome == true {
					log.Printf("Hashtable successfully updated to %v.\n",  value)
				} else {
					log.Println("Update unsuccessful, please try again.")
				}
				
			} else if (textChoice == "get") {
				// println("Enter the key of the value you would like to retireve (integers only!): ")
				// scanner.Scan()
				// text := scanner.Text()
				// key, err := strconv.Atoi(text)
				// if err != nil {
				// 	log.Println("Client: Could not convert key to integer: ", err)
				// 	continue
				// }
				
				// getReq := &service.RetrieveRequest{
				// 	Id:   int32(key),
				// }

				//result := Retrieve(getReq) 
				result := Retrieve(&service.RetrieveRequest{})
				log.Printf("Client: Value of: ", int(result))
				fmt.Printf("Client: Value of: ",int(result))
				//log.Printf("Client: Value of "key %s: %v \n",text," int(result))
				//fmt.Printf("Value of key %s: %v \n",text, int(result))
			} else {
				log.Println("Incorrect input. ")
				fmt.Println("Incorrect input. ")
			}
		}
	}()

	for {}
}

//If function returns an error, redial to other front-end and try again
func Update(hashUpt *service.UpdateRequest) (*service.UpdateResponse) {
	fmt.Println("inside update")
	result, err := server.Update(context.Background(), hashUpt) 
	if err != nil {
		log.Printf("Client %s hashUpdate failed:%s. \n Redialing and retrying. \n", connection.Target(), err)
		Redial()
		return Update(hashUpt)
	}
	return result
}

func Retrieve(getRsqt *service.RetrieveRequest) (int32) {
	fmt.Println("inside retrieve")
	result, err := server.Retrieve(context.Background(), getRsqt)
	if err != nil {
		log.Printf("Client %s get request failed: %s", connection.Target(), err)
		Redial()
		return Retrieve(getRsqt)
	}

	if reflect.ValueOf(result.Outcome).Kind() != reflect.ValueOf(int32(5)).Kind() {
		return 0
	}
	return result.Outcome
}

//In the case of losing connection - alternates between predefined front-
func Redial() {
	var port string
	if connection.Target()[len(connection.Target())-1:] == "1" {
		port =  ":4000"
	} else {
		port = ":4001"
	}

	conn, err := grpc.Dial(port, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Client: Unable to connect to port %s: %v", port, err)
	}

	connection = conn
	server = service.NewServiceClient(connection) //creates a connection with an FE server
}


func setLogClient() *os.File {
	f, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)
	return f
}