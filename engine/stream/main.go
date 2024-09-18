package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

// Define constants and variables
const (
	webPort = ":8080"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	fmt.Printf("Starting server at http://localhost%s\n", webPort)
	log.Fatal(http.ListenAndServe(webPort, nil))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket Upgrade Error:", err)
		return
	}
	defer conn.Close()

	peerConnection, err := createPeerConnection()
	if err != nil {
		log.Println("Failed to create PeerConnection:", err)
		return
	}

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		if err := conn.WriteJSON(map[string]interface{}{
			"type":      "candidate",
			"candidate": candidate.ToJSON(),
		}); err != nil {
			log.Println("Failed to send ICE candidate:", err)
		}
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket Read Error:", err)
			break
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Println("JSON Unmarshal Error:", err)
			continue
		}

		switch msg["type"] {
		case "offer":
			offer := webrtc.SessionDescription{
				Type: webrtc.SDPTypeOffer,
				SDP:  msg["sdp"].(string),
			}

			if err := peerConnection.SetRemoteDescription(offer); err != nil {
				log.Println("Failed to set remote description:", err)
				return
			}

			answer, err := peerConnection.CreateAnswer(nil)
			if err != nil {
				log.Println("Failed to create answer:", err)
				return
			}
			if err := peerConnection.SetLocalDescription(answer); err != nil {
				log.Println("Failed to set local description:", err)
				return
			}

			if err := conn.WriteJSON(map[string]interface{}{
				"type": "answer",
				"sdp":  answer.SDP,
			}); err != nil {
				log.Println("Failed to send answer:", err)
			}

		case "candidate":
			candidate := webrtc.ICECandidateInit{
				Candidate: msg["candidate"].(string),
			}
			if err := peerConnection.AddICECandidate(candidate); err != nil {
				log.Println("Failed to add ICE candidate:", err)
			}

		case "start-recording":
			log.Println("Starting recording")

		case "stop-recording":
			log.Println("Stopping recording")
		}
	}
}

func createPeerConnection() (*webrtc.PeerConnection, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	webmFile, err := os.Create("output.webm")
	if err != nil {
		return nil, err
	}

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			if _, err := webmFile.Write(msg.Data); err != nil {
				log.Println("Error writing to WebM file:", err)
			}
		})
	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())

		if connectionState == webrtc.ICEConnectionStateDisconnected {
			fmt.Println("Peer disconnected")
			if err := webmFile.Close(); err != nil {
				log.Println("Error closing WebM file:", err)
			}
			os.Exit(0)
		}
	})

	return peerConnection, nil
}
