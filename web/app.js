document.getElementById("join-btn").addEventListener("click", joinSession)
document.getElementById("start-btn").addEventListener("click", startRecording)
document.getElementById("stop-btn").addEventListener("click", stopRecording)

let ws
let localStream
let peerConnection
let dataChannel
let mediaRecorder
let isMuted = false
let isVideoStopped = false
let isDataChannelOpen = false
let chunkQueue = []

async function joinSession() {
  console.log("Joining session...")
  const name = document.getElementById("name").value
  if (!name) {
    console.warn("Name not provided")
    alert("Please enter your name")
    return
  }

  document.getElementById("join-screen").style.display = "none"
  document.getElementById("participant-view").style.display = "block"

  peerConnection = new RTCPeerConnection({
    iceServers: [{ urls: "stun:stun.l.google.com:19302" }]
  })
  console.log("RTCPeerConnection created")

  dataChannel = peerConnection.createDataChannel("videoChannel", { 
    ordered: true,
    maxRetransmits: 3
  })
  dataChannel.binaryType = "arraybuffer"
  console.log("Data channel created")

  dataChannel.onopen = () => {
    console.log("Data channel opened")
    isDataChannelOpen = true
    sendQueuedChunks()
  }

  dataChannel.onclose = () => {
    console.log("Data channel closed")
    isDataChannelOpen = false
  }

  dataChannel.onerror = (error) => {
    console.error("Data channel error:", error)
    isDataChannelOpen = false
  }

  // ws = new WebSocket(`ws://${window.location.host}/ws`)
  ws = new WebSocket(`ws://localhost:8080/ws`)

  ws.onopen = async () => {
    console.log("WebSocket connection opened")
    try {
      localStream = await navigator.mediaDevices.getUserMedia({
        video: {
          width: { ideal: 1280 },
          height: { ideal: 720 },
          frameRate: { ideal: 30 },
          aspectRatio: 16 / 9
        },
        audio: true
      })
      console.log("Local media stream obtained")

      const localVideo = document.createElement("video")
      localVideo.srcObject = localStream
      localVideo.autoplay = true
      localVideo.muted = true
      document.getElementById("videos").appendChild(localVideo)
      console.log("Local video element created and added to DOM")

      mediaRecorder = new MediaRecorder(localStream, {
        mimeType: 'video/webm;codecs=vp8,opus',
        videoBitsPerSecond: 1000000 // 1 Mbps
      })
      console.log("MediaRecorder created")

      mediaRecorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          const chunk = event.data
          if (isDataChannelOpen) {
            sendVideoChunk(chunk)
          } else {
            chunkQueue.push(chunk)
            if (chunkQueue.length > 100) {
              chunkQueue.shift() // Remove oldest chunk if queue gets too large
            }
          }
        }
      }

      mediaRecorder.start(100) // Start recording and send data every 100ms
      console.log("MediaRecorder started")

      localStream.getTracks().forEach(track => peerConnection.addTrack(track, localStream))

      const offer = await peerConnection.createOffer()
      await peerConnection.setLocalDescription(offer)
      console.log("Local description set")

      ws.send(JSON.stringify({ type: "offer", sdp: offer.sdp }))
      console.log("Offer sent through WebSocket")
    } catch (err) {
      console.error("Error during WebRTC setup:", err)
    }
  }

  ws.onmessage = async (message) => {
    const data = JSON.parse(message.data)
    console.log("Received message:", data.type)
    try {
      if (data.type === "answer") {
        await peerConnection.setRemoteDescription(
          new RTCSessionDescription(data)
        )
        console.log("Remote description set")
      } else if (data.type === "candidate") {
        if (data.candidate) {
          await peerConnection.addIceCandidate(
            new RTCIceCandidate(data.candidate)
          )
          console.log("ICE candidate added")
        }
      }
    } catch (err) {
      console.error("Error handling WebSocket message:", err)
    }
  }

  peerConnection.onicecandidate = (event) => {
    if (event.candidate) {
      ws.send(
        JSON.stringify({
          type: "candidate",
          candidate: event.candidate.candidate
        })
      )
      console.log("ICE candidate sent")
    }
  }

  peerConnection.oniceconnectionstatechange = function () {
    console.log(
      "[STATE] ICE Connection State has changed:",
      peerConnection.iceConnectionState
    )

    if (
      peerConnection.iceConnectionState === "connected" ||
      peerConnection.iceConnectionState === "completed"
    ) {
      console.log("[STATE] WebRTC connection successfully established.")
    }

    if (
      peerConnection.iceConnectionState === "failed" ||
      peerConnection.iceConnectionState === "disconnected"
    ) {
      console.log("[STATE] WebRTC connection failed or disconnected.")
      isDataChannelOpen = false
    }
  }

  ws.onclose = (event) => {
    console.log("WebSocket closed:", event)
    isDataChannelOpen = false
  }

  ws.onerror = (error) => {
    console.error("WebSocket error:", error)
    isDataChannelOpen = false
  }
}

function sendVideoChunk(chunk) {
  const reader = new FileReader()
  reader.onload = function() {
    try {
      const chunkSize = 16384 // 16 KB chunks
      const arrayBuffer = reader.result
      for (let i = 0; i < arrayBuffer.byteLength; i += chunkSize) {
        const slice = arrayBuffer.slice(i, i + chunkSize)
        dataChannel.send(slice)
      }
      console.log("Video chunk sent through data channel")
    } catch (error) {
      console.error("Error sending data through channel:", error)
      chunkQueue.push(chunk)
    }
  }
  reader.readAsArrayBuffer(chunk)
}

function sendQueuedChunks() {
  while (chunkQueue.length > 0 && isDataChannelOpen) {
    const chunk = chunkQueue.shift()
    sendVideoChunk(chunk)
  }
}

function startRecording() {
  console.log("Starting recording...")
  if (mediaRecorder && mediaRecorder.state === "paused") {
    mediaRecorder.resume()
    console.log("MediaRecorder resumed")
  }
  ws.send(JSON.stringify({ type: "start-recording" }))
  console.log("Start recording message sent")
}

function stopRecording() {
  console.log("Stopping recording...")
  if (mediaRecorder && mediaRecorder.state === "recording") {
    mediaRecorder.pause()
    console.log("MediaRecorder paused")
  }
  ws.send(JSON.stringify({ type: "stop-recording" }))
  console.log("Stop recording message sent")
}
