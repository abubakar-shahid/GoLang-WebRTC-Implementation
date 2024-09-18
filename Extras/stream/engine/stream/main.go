// main.go

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

const (
	webPort = ":8080"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var (
	videoDataChannel *webrtc.DataChannel
	webmFile         *os.File
	webmMutex        sync.Mutex
)

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
			sdp, ok := msg["sdp"].(string)
			if !ok {
				log.Println("Invalid SDP type")
				continue
			}
			handleOffer(peerConnection, conn, sdp)
		case "candidate":
			candidate, ok := msg["candidate"].(map[string]interface{})
			if !ok {
				log.Println("Invalid candidate type")
				continue
			}
			handleCandidate(peerConnection, candidate)
		case "start-recording":
			startRecording()
			conn.WriteJSON(map[string]string{"type": "recording-started"})
		case "stop-recording":
			stopRecording()
			conn.WriteJSON(map[string]string{"type": "recording-stopped"})
		}
	}
}

func handleOffer(peerConnection *webrtc.PeerConnection, conn *websocket.Conn, sdp string) {
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
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
}

func handleCandidate(peerConnection *webrtc.PeerConnection, candidateInit map[string]interface{}) {
	candidate := webrtc.ICECandidateInit{
		Candidate: candidateInit["candidate"].(string),
		SDPMid:    func() *string { if v, ok := candidateInit["sdpMid"].(string); ok { return &v }; return nil }(),
		SDPMLineIndex: func() *uint16 {
			index, ok := candidateInit["sdpMLineIndex"].(float64)
			if !ok {
				return nil
			}
			indexUint := uint16(index)
			return &indexUint
		}(),
		UsernameFragment: func() *string { if v, ok := candidateInit["usernameFragment"].(string); ok { return &v }; return nil }(),
	}
	if err := peerConnection.AddICECandidate(candidate); err != nil {
		log.Println("Failed to add ICE candidate:", err)
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

	peerConnection.OnDataChannel(handleDataChannel)

	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		fmt.Println("[STATE] ICE Connection State has changed:", state.String())

		if state == webrtc.ICEConnectionStateConnected || state == webrtc.ICEConnectionStateCompleted {
			fmt.Println("[STATE] WebRTC connection successfully established.")
		}

		if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateDisconnected {
			fmt.Println("[STATE] Connection failed or disconnected.")
			stopRecording()

			if err := peerConnection.Close(); err != nil {
				log.Println("Error closing peer connection:", err)
			}

			os.Exit(0)
		}
	})

	return peerConnection, nil
}

func handleDataChannel(d *webrtc.DataChannel) {
	fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

	if d.Label() == "video" {
		videoDataChannel = d
	}

	d.OnOpen(func() {
		fmt.Printf("Data channel '%s'-'%d' open.\n", d.Label(), d.ID())
	})

	d.OnClose(func() {
		fmt.Printf("Data channel '%s'-'%d' closed.\n", d.Label(), d.ID())
	})

	d.OnMessage(func(msg webrtc.DataChannelMessage) {
		if d.Label() == "video" {
			handleVideoData(msg.Data)
		}
	})
}

func handleVideoData(data []byte) {
	webmMutex.Lock()
	defer webmMutex.Unlock()

	if webmFile == nil {
		log.Println("No recording in progress")
		return
	}

	if _, err := webmFile.Write(data); err != nil {
		log.Println("Error writing video frame:", err)
	}
}

func startRecording() {
	webmMutex.Lock()
	defer webmMutex.Unlock()

	if webmFile != nil {
		log.Println("Recording is already in progress")
		return
	}

	var err error
	webmFile, err = os.Create("output.webm")
	if err != nil {
		log.Println("Error creating WebM file:", err)
		return
	}

	log.Println("Started recording")
}

func stopRecording() {
	webmMutex.Lock()
	defer webmMutex.Unlock()

	if webmFile == nil {
		log.Println("No recording in progress")
		return
	}

	if err := webmFile.Close(); err != nil {
		log.Println("Error closing WebM file:", err)
	}

	webmFile = nil
	log.Println("Stopped recording")
}
