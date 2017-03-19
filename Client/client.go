package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pb "github.com/taylorflatt/go-chat"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// The IP is hardcoded now. But eventually it will not be.
// Will need to reference Interfaces().
const (
	ip = "localhost"
	//port = 12021
)

// RandInt32 generates a random int32 between two values.
func RandInt32(min int32, max int32) int32 {
	rand.Seed(time.Now().Unix())
	return min + rand.Int31n(max-min)
}

func CheckError(err error) {
	if err != nil {
		fmt.Print(err)
	}
}

func ExitChat(c pb.ChatClient, uName string) {
	c.UnRegister(context.Background(), &pb.ClientInfo{Sender: uName})
	os.Exit(1)
}

func ControlExitEarly(w chan os.Signal, c pb.ChatClient, uName string) {

	signal.Notify(w, syscall.SIGINT, syscall.SIGTERM)

	sig := <-w
	fmt.Print(sig)
	fmt.Println(" used.")
	fmt.Println("Exiting chat application.")

	if sig == os.Interrupt {
		ExitChat(c, uName)
	}
}

func ControlExitLate(w chan os.Signal, c pb.ChatClient, uName string) {

	ExitChat(c, uName)
}

func main() {

	// Read in the user's command.
	r := bufio.NewReader(os.Stdin)

	// username, groupname
	var uName string
	var gName string

	// Read the server address
	// DEBUG ONLY:
	address := "localhost:12021"
	// UNCOMMENT AFTER DEBUG
	//fmt.Print("Please specify the server IP: ")
	//t, _ := r.ReadString('\n')
	//t = strings.TrimSpace(t)
	//ts := strings.Split(t, ":")
	//sip := ts[0]
	//sport := ts[1]
	//address := sip + ":" + sport
	// END UNCOMMENT

	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())

	if err != nil {
		log.Fatalf("Could not connect: %v", err)
	} else {
		fmt.Printf("\nYou have successfully connected to %s! To disconnect, hit ctrl+c or type !exit.\n", address)
	}

	// Close the connection after main returns.
	defer conn.Close()

	// Create the client
	c := pb.NewChatClient(conn)

	// Register the client with the server.
	for {
		fmt.Printf("Enter your username: ")
		tu, err := r.ReadString('\n')
		if err != nil {
			fmt.Print(err)
		}
		uName = strings.TrimSpace(tu)

		_, err = c.Register(context.Background(), &pb.ClientInfo{Sender: uName})

		if err == nil {
			fmt.Println("Your username: " + uName)
			w := make(chan os.Signal, 1)
			go ControlExitEarly(w, c, uName)
			break
		} else {
			fmt.Print(err)
		}
	}

	fmt.Println("You are now chatting in " + gName)
	addSpacing(1)

	stream, serr := c.RouteChat(context.Background())
	// Send some fake message to myself
	stream.Send(&pb.ChatMessage{Sender: uName, Receiver: gName, Message: ""})
	stream.Send(&pb.ChatMessage{Sender: uName, Receiver: gName, Message: uName + " joined chat!\n"})

	if serr != nil {
		fmt.Print(serr)
	} else {

		sQueue := make(chan pb.ChatMessage, 100)
		go listenToClient(sQueue, r, uName, gName)

		inbox := make(chan pb.ChatMessage, 100)
		go receiveMessages(stream, inbox)

		for {
			select {
			case toSend := <-sQueue:
				stream.Send(&toSend)
			case received := <-inbox:
				fmt.Printf("%s> %s", received.Sender, received.Message)
			}
		}

		//stream.Send(&pb.ChatMessage{Sender: uName, Receiver: gName})
	}
}

func addSpacing(n int) {
	for i := 0; i <= n; i++ {
		fmt.Println()
	}
}

func listenToClient(sQueue chan pb.ChatMessage, reader *bufio.Reader, uName string, gName string) {
	for {
		msg, _ := reader.ReadString('\n')
		sQueue <- pb.ChatMessage{Sender: uName, Message: msg, Receiver: gName}
	}
}

// Check here if the msg coming in is from itself (sender == uName)
func receiveMessages(stream pb.Chat_RouteChatClient, inbox chan pb.ChatMessage) {
	for {
		msg, _ := stream.Recv()
		inbox <- *msg
	}
}
