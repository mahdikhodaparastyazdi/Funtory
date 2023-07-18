package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"database/sql"
	"net/http"

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
	err       error
	db        *sql.DB
	container *sqlstore.Container
	dbLog     waLog.Logger
	client    *whatsmeow.Client
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
	defer db.Close()
}

func connectSessionDB() {
	container, err = sqlstore.New("postgres", "host=127.0.0.1 dbname=mydb user=myuser password=1234 port=5432", dbLog)
	if err != nil {
		panic(err)
	}
}

func initial() {
	connectDB()
	connectSessionDB()
}

func main() {

	dbLog = waLog.Stdout("Database", "DEBUG", true)
	initial()
	fmt.Println(db)
	http.HandleFunc("/connect", handleRequest)

	log.Println("Server started on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	client.Disconnect()
}

func handleRequest(w http.ResponseWriter, r *http.Request) {

	userIdStr := r.URL.Query().Get("userId")
	_, err := strconv.Atoi(userIdStr)
	if err != nil {
		http.Error(w, "Invalid userId", http.StatusBadRequest)
		return
	}

	jidString, check := findJid(userIdStr)
	if check {
		var jid types.JID
		err = json.Unmarshal([]byte(jidString), &jid)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		conectByJID(jid)
	}
}

func findJid(userId string) (string, bool) {
	var jidString string
	err = db.QueryRow("SELECT jid FROM users WHERE id = $1", userId).Scan(&jidString)
	if err == sql.ErrNoRows {
		return "User not found", false
	} else if err != nil {
		return "Database error", false
	}
	return jidString, true
}

func conectByJID(jid types.JID) {
	deviceStore, err := container.GetDevice(jid)
	if err != nil {
		return
	}
	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)
	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			return
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
			return
		}
	}

}
