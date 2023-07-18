package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"database/sql"
	"net/http"

	"WHATSMEOW/api"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	err error
	db  *sql.DB
)

type SecondRequestResponse struct {
	JID string `json:"jid"`
}

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		fmt.Println("Received a message!", v.Message.GetConversation())
	}
}

func connectDB() {
	connectionString := "postgres://user:password@localhost/dbname?sslmode=disable" // Replace with your database connection details
	db, err = sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatal(err)
	}
}
func initial() {
	connectDB()
	defer db.Close()
}
func main() {

	dbLog := waLog.Stdout("Database", "DEBUG", true)
	initial()
	fmt.Println(db)
	http.HandleFunc("/connect", api.HandleRequest)

	log.Println("Server started on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))

	// psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	// Make sure you add appropriate DB connector imports, e.g. github.com/mattn/go-sqlite3 for SQLite
	container, err := sqlstore.New("postgres", "host=127.0.0.1 dbname=mydb user=myuser password=1234 port=5432", dbLog)
	if err != nil {
		panic(err)
	}
	var jid types.JID
	fmt.Print(jid)

	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	deviceStore.ID.User = ""
	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)
	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("QR code:", evt.Code)
				qrterminal.Generate(evt.Code, qrterminal.M, os.Stdout)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}
