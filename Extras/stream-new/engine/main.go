package main

import (
	"encoding/json"
	"log"
	"net/http"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var pc *webrtc.PeerConnection

func handleWebSocketConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal("Error while connecting to WebSocket:", err)
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error while reading message:", err)
			return
		}

		var signal Signal
		if err := json.Unmarshal(msg, &signal); err != nil {
			log.Println("Error unmarshalling message:", err)
			return
		}

		switch signal.Type {
		case "offer":
			handleOffer(conn, signal)
		case "answer":
			handleAnswer(conn, signal)
		case "candidate":
			handleCandidate(signal)
		}
	}
}

func handleOffer(conn *websocket.Conn, signal Signal) {
	offer := webrtc.SessionDescription{}
	if err := json.Unmarshal([]byte(signal.SDP), &offer); err != nil {
		log.Println("Error unmarshalling offer:", err)
		return
	}

	var err error
	pc, err = webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		log.Println("Error creating peer connection:", err)
		return
	}

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			signal := Signal{
				Type:      "candidate",
				Candidate: c.ToJSON().Candidate,
			}
			msg, _ := json.Marshal(signal)
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	})

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("Track received: %s", track.Kind())
	})

	if err := pc.SetRemoteDescription(offer); err != nil {
		log.Println("Error setting remote description:", err)
		return
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Println("Error creating answer:", err)
		return
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		log.Println("Error setting local description:", err)
		return
	}

	signal = Signal{
		Type: "answer",
		SDP:  answer.SDP,
	}
	msg, _ := json.Marshal(signal)
	conn.WriteMessage(websocket.TextMessage, msg)
}

func handleAnswer(conn *websocket.Conn, signal Signal) {
	answer := webrtc.SessionDescription{}
	if err := json.Unmarshal([]byte(signal.SDP), &answer); err != nil {
		log.Println("Error unmarshalling answer:", err)
		return
	}

	if err := pc.SetRemoteDescription(answer); err != nil {
		log.Println("Error setting remote description:", err)
	}
}

func handleCandidate(signal Signal) {
	if pc != nil {
		candidate := webrtc.ICECandidateInit{
			Candidate: signal.Candidate,
		}
		pc.AddICECandidate(candidate)
	}
}

type Signal struct {
	Type      string `json:"type"`
	SDP       string `json:"sdp"`
	Candidate string `json:"candidate"`
}

func main() {
	http.HandleFunc("/ws", handleWebSocketConnection)
	log.Fatal(http.ListenAndServe(":8081", nil))
}