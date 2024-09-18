package main

import (
    "fmt"
    // "io"
    "log"
    "os"
    "os/signal"
    "time"
    "github.com/go-audio/audio"
    "github.com/go-audio/wav"
    "github.com/gorilla/websocket"
    "github.com/pion/webrtc/v3"
)

// SignalMessage represents the structure of messages exchanged with the signaling server
type SignalMessage struct {
    Type      string                     `json:"type"`
    SDP       *webrtc.SessionDescription `json:"sdp,omitempty"`
    Candidate *webrtc.ICECandidateInit   `json:"candidate,omitempty"`
}

// logWithTimestamp logs a message with a timestamp
func logWithTimestamp(message string) {
    log.Printf("[%s] %s", time.Now().Format("2006-01-02 15:04:05.000"), message)
}

func main() {
    log.SetFlags(log.LstdFlags | log.Lmicroseconds)
    logWithTimestamp("Starting WebRTC audio receiver...")

    // Connect to the signaling server
    serverURL := "ws://localhost:3000"
    logWithTimestamp(fmt.Sprintf("Connecting to signaling server at: %s", serverURL))
    conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
    if err != nil {
        logWithTimestamp(fmt.Sprintf("WebSocket connection failed: %v", err))
        log.Fatal(err)
    }
    defer conn.Close()
    logWithTimestamp("Successfully connected to signaling server")

    // Create a new WebRTC API
    logWithTimestamp("Creating new WebRTC peer connection...")
    peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
        ICEServers: []webrtc.ICEServer{
            {URLs: []string{"stun:stun.l.google.com:19302"}},
        },
    })
    if err != nil {
        logWithTimestamp(fmt.Sprintf("Failed to create peer connection: %v", err))
        log.Fatal(err)
    }
    defer peerConnection.Close()
    logWithTimestamp("WebRTC peer connection created successfully")

    // Setup signaling and ICE handling
    setupSignalHandlers(peerConnection, conn)

    // Call the handleTrack function to handle incoming audio tracks
    handleTrack(peerConnection)

    // Wait for interrupt signal to gracefully close the connection
    interrupt := make(chan os.Signal, 1)
    signal.Notify(interrupt, os.Interrupt)
    logWithTimestamp("Waiting for interrupt signal...")
    <-interrupt
    logWithTimestamp("Interrupt received, closing connection...")
}

// setupSignalHandlers sets up the signaling and track handlers for the peer connection
func setupSignalHandlers(peerConnection *webrtc.PeerConnection, conn *websocket.Conn) {
    // Log changes in peer connection states
    peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
        logWithTimestamp(fmt.Sprintf("Peer Connection State has changed: %s", s.String()))
        if s == webrtc.PeerConnectionStateConnected {
            logWithTimestamp("Peer Connection is now connected, ready to receive tracks.")
        }
    })

    peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
        logWithTimestamp(fmt.Sprintf("ICE Connection State has changed: %s", connectionState.String()))
    })

    peerConnection.OnSignalingStateChange(func(signalState webrtc.SignalingState) {
        logWithTimestamp(fmt.Sprintf("Signaling State has changed: %s", signalState.String()))
    })

    // Handle incoming ICE candidates from the Web client
    peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
        if candidate == nil {
            logWithTimestamp("Finished gathering ICE candidates")
            return
        }
        logWithTimestamp(fmt.Sprintf("Received ICE candidate: %v", candidate))
        candidateJSON := candidate.ToJSON()
        message := SignalMessage{
            Type:      "candidate",
            Candidate: &candidateJSON,
        }
        err := conn.WriteJSON(message)
        if err != nil {
            logWithTimestamp(fmt.Sprintf("Error sending ICE candidate: %v", err))
        } else {
            logWithTimestamp("ICE candidate sent to web client")
        }
    })

    // Handle incoming SDP and ICE candidates messages from signaling server
    go func() {
        for {
            logWithTimestamp("Waiting for WebSocket message...")
            var message SignalMessage
            err := conn.ReadJSON(&message)
            if err != nil {
                logWithTimestamp(fmt.Sprintf("Error reading message: %v", err))
                return
            }
            logWithTimestamp(fmt.Sprintf("Received message of type: %s", message.Type))
            handleSignalingMessage(peerConnection, conn, &message)
        }
    }()
}

// handleSignalingMessage handles the incoming signaling messages
func handleSignalingMessage(peerConnection *webrtc.PeerConnection, conn *websocket.Conn, message *SignalMessage) {
    switch message.Type {
    case "offer":
        logWithTimestamp("Processing SDP offer...")
        err := peerConnection.SetRemoteDescription(*message.SDP)
        if err != nil {
            logWithTimestamp(fmt.Sprintf("Error setting remote description: %v", err))
            return
        }
        logWithTimestamp("Remote description set successfully")
        logWithTimestamp("Creating answer...")
        answer, err := peerConnection.CreateAnswer(nil)
        if err != nil {
            logWithTimestamp(fmt.Sprintf("Error creating answer: %v", err))
            return
        }
        logWithTimestamp("Answer created successfully")
        logWithTimestamp("Setting local description...")
        err = peerConnection.SetLocalDescription(answer)
        if err != nil {
            logWithTimestamp(fmt.Sprintf("Error setting local description: %v", err))
            return
        }
        logWithTimestamp("Local description set successfully")
        logWithTimestamp("Sending answer to web client...")
        err = conn.WriteJSON(SignalMessage{Type: "answer", SDP: &answer})
        if err != nil {
            logWithTimestamp(fmt.Sprintf("Error sending answer: %v", err))
            return
        }
        logWithTimestamp("Answer sent successfully")
    case "candidate":
        logWithTimestamp("Processing ICE candidate...")
        if message.Candidate != nil {
            err := peerConnection.AddICECandidate(*message.Candidate)
            if err != nil {
                logWithTimestamp(fmt.Sprintf("Error adding ICE candidate: %v", err))
                return
            }
            logWithTimestamp("ICE candidate added successfully")
        }
    }
}

func handleTrack(peerConnection *webrtc.PeerConnection) {
    // When an incoming track is detected, handle the track
    peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
        logWithTimestamp(fmt.Sprintf("New incoming track received of type: %s", track.Kind().String()))

        // Check if the incoming track is audio
        if track.Kind() == webrtc.RTPCodecTypeAudio {
            logWithTimestamp("Handling incoming audio track...")

            // Prepare to write incoming audio to a file
            outFile, err := os.Create("received_audio.wav")
            if err != nil {
                logWithTimestamp(fmt.Sprintf("Error creating audio file: %v", err))
                return
            }
            defer outFile.Close()

            // Set up WAV encoder
            enc := wav.NewEncoder(outFile, 48000, 16, 1, 1) // Example for mono audio, 48kHz sample rate

            // Receive audio packets
            audioBuf := &audio.IntBuffer{Data: make([]int, 0), Format: &audio.Format{SampleRate: 48000, NumChannels: 1}}
            for {
                // Read RTP packets
                pkt, _, err := track.ReadRTP()
                if err != nil {
                    logWithTimestamp(fmt.Sprintf("Error reading RTP packets: %v", err))
                    break
                }

                // Decode audio data and append to buffer
                for i := 0; i < len(pkt.Payload); i += 2 {
                    sample := int16(pkt.Payload[i+1])<<8 | int16(pkt.Payload[i])
                    audioBuf.Data = append(audioBuf.Data, int(sample))
                }

                // Write data to WAV file in chunks
                if err := enc.Write(audioBuf); err != nil {
                    logWithTimestamp(fmt.Sprintf("Error writing audio data to file: %v", err))
                    break
                }
                audioBuf.Data = audioBuf.Data[:0] // Reset buffer
            }

            // Finalize the WAV file
            if err := enc.Close(); err != nil {
                logWithTimestamp(fmt.Sprintf("Error closing WAV file encoder: %v", err))
            }
        } else {
            logWithTimestamp("Received non-audio track, ignoring...")
        }
    })
}


// func handleTrack(peerConnection *webrtc.PeerConnection) {
//     logWithTimestamp("Setting up track handler...")

//     outputFile, err := os.Create("received_audio.wav")
//     if err != nil {
//         logWithTimestamp(fmt.Sprintf("Error creating WAV file: %v", err))
//         log.Fatal(err)
//     }
//     wavEncoder := wav.NewEncoder(outputFile, 48000, 16, 1, 1)

//     peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
//         logWithTimestamp(fmt.Sprintf("Received track with codec: %s", track.Codec().MimeType))

//         if track.Kind() != webrtc.RTPCodecTypeAudio {
//             logWithTimestamp("Non-audio track received, ignoring...")
//             return
//         }

//         const targetSampleSize = 480 // Number of samples (shorts) we want to buffer
//         buffer := make([]int, 0, targetSampleSize)

//         for {
//             rtp, _, readErr := track.ReadRTP()
//             if readErr != nil {
//                 if readErr == io.EOF {
//                     logWithTimestamp("End of audio stream")
//                     break
//                 }
//                 logWithTimestamp(fmt.Sprintf("Error reading RTP packet: %v", readErr))
//                 break
//             }

//             // Convert RTP payload to samples
//             for i := 0; i < len(rtp.Payload)-1; i += 2 {
//                 sample := int16(rtp.Payload[i]) | int16(rtp.Payload[i+1])<<8
//                 buffer = append(buffer, int(sample))

//                 // When buffer reaches targetSampleSize, write to WAV file
//                 if len(buffer) == targetSampleSize {
//                     logWithTimestamp(fmt.Sprintf("Writing %d samples to WAV file", len(buffer)))
//                     if err := wavEncoder.Write(&audio.IntBuffer{Data: buffer, Format: &audio.Format{SampleRate: 48000, NumChannels: 1}}); err != nil {
//                         logWithTimestamp(fmt.Sprintf("Error writing to WAV file: %v", err))
//                         return
//                     }
//                     buffer = buffer[:0] // Clear buffer
//                 }
//             }

//             // Handle the last byte if the payload length is odd
//             if len(rtp.Payload)%2 != 0 {
//                 lastByte := int16(rtp.Payload[len(rtp.Payload)-1])
//                 buffer = append(buffer, int(lastByte))
//             }
//         }

//         // Write any remaining samples in the buffer to the WAV file
//         if len(buffer) > 0 {
//             logWithTimestamp(fmt.Sprintf("Writing remaining %d samples to WAV file", len(buffer)))
//             if err := wavEncoder.Write(&audio.IntBuffer{Data: buffer, Format: &audio.Format{SampleRate: 48000, NumChannels: 1}}); err != nil {
//                 logWithTimestamp(fmt.Sprintf("Error writing to WAV file: %v", err))
//             }
//         }

//         logWithTimestamp("Closing WAV encoder")
//         if err := wavEncoder.Close(); err != nil {
//             logWithTimestamp(fmt.Sprintf("Error closing WAV encoder: %v", err))
//         }

//         logWithTimestamp("Closing output file")
//         if err := outputFile.Close(); err != nil {
//             logWithTimestamp(fmt.Sprintf("Error closing output file: %v", err))
//         }
//     })
// }
