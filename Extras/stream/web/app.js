// app.js

document.getElementById("join-btn").addEventListener("click", joinSession);
document.getElementById("start-btn").addEventListener("click", startRecording);
document.getElementById("stop-btn").addEventListener("click", stopRecording);

let ws;
let localStream;
let peerConnection;
let videoDataChannel;
let isWebSocketConnected = false;
let isRecording = false;
let mediaRecorder;
let recordedChunks = [];
let isVideoDataChannelOpen = false;

async function joinSession() {
  const name = document.getElementById("name").value;
  if (!name) {
    alert("Please enter your name");
    return;
  }

  document.getElementById("join-screen").style.display = "none";
  document.getElementById("participant-view").style.display = "block";

  await setupWebRTCConnection();
}

async function setupWebRTCConnection() {
  peerConnection = new RTCPeerConnection({
    iceServers: [{ urls: "stun:stun.l.google.com:19302" }]
  });

  peerConnection.ondatachannel = handleDataChannel;

  try {
    ws = new WebSocket(`ws://localhost:8080/ws`);

    ws.onopen = async () => {
      isWebSocketConnected = true;
      await setupLocalStream();
    };

    ws.onmessage = async (message) => {
      const data = JSON.parse(message.data);
      try {
        if (data.type === "answer") {
          await peerConnection.setRemoteDescription(new RTCSessionDescription(data));
        } else if (data.type === "candidate" && data.candidate) {
          await peerConnection.addIceCandidate(new RTCIceCandidate(data.candidate));
        }
      } catch (err) {
        console.error("Error handling WebSocket message:", err);
      }
    };

    ws.onclose = (event) => {
      console.log("WebSocket closed:", event);
      isWebSocketConnected = false;
      cleanupConnection();
      alert("Connection to server lost. Please refresh the page to reconnect.");
    };

    ws.onerror = (error) => {
      console.error("WebSocket error:", error);
      isWebSocketConnected = false;
      alert("Error connecting to server. Please check your internet connection and try again.");
    };

    peerConnection.onicecandidate = (event) => {
      if (event.candidate && isWebSocketConnected) {
        ws.send(JSON.stringify({
          type: "candidate",
          candidate: event.candidate.toJSON()
        }));
      }
    };

    peerConnection.oniceconnectionstatechange = () => {
      console.log("[STATE] ICE Connection State has changed:", peerConnection.iceConnectionState);

      if (peerConnection.iceConnectionState === "checking") {
        console.log("[STATE] ICE Connection State has changed: checking");
      } else if (peerConnection.iceConnectionState === "connected") {
        console.log("[STATE] ICE Connection State has changed: connected");
        console.log("[STATE] WebRTC connection successfully established.");
      }

      if (peerConnection.iceConnectionState === "failed" || peerConnection.iceConnectionState === "disconnected") {
        console.log("[STATE] WebRTC connection failed or disconnected.");
        alert("WebRTC connection failed. Please refresh the page to try again.");
        cleanupConnection();
      }
    };

  } catch (error) {
    console.error("Error setting up WebRTC connection:", error);
    alert("Failed to set up connection. Please refresh the page and try again.");
  }
}

function handleDataChannel(event) {
  const channel = event.channel;
  console.log(`New DataChannel ${channel.label} ${channel.id}`);

  if (channel.label === "video") {
    videoDataChannel = channel;
    videoDataChannel.onopen = () => {
      console.log("Video Data Channel is open.");
      isVideoDataChannelOpen = true;
    };
    videoDataChannel.onclose = () => {
      console.log("Video Data Channel is closed.");
      isVideoDataChannelOpen = false;
    };

    videoDataChannel.onmessage = (event) => {
      console.log("Received data:", event.data);
    };
  }
}

async function setupLocalStream() {
  try {
    localStream = await navigator.mediaDevices.getUserMedia({
      video: {
        width: { ideal: 640 },
        height: { ideal: 480 },
        frameRate: { ideal: 15 },
        aspectRatio: 4 / 3
      },
      audio: true
    });

    const localVideo = document.createElement("video");
    localVideo.srcObject = localStream;
    localVideo.autoplay = true;
    localVideo.muted = true;
    document.getElementById("videos").appendChild(localVideo);

    // Create and open the video data channel
    videoDataChannel = peerConnection.createDataChannel("video");
    videoDataChannel.onopen = () => {
      console.log("Video Data Channel is open.");
      isVideoDataChannelOpen = true;
    };
    videoDataChannel.onclose = () => {
      console.log("Video Data Channel is closed.");
      isVideoDataChannelOpen = false;
    };

    startSendingVideoData(localStream);

    const offer = await peerConnection.createOffer();
    await peerConnection.setLocalDescription(offer);

    ws.send(JSON.stringify({ type: "offer", sdp: offer.sdp }));
  } catch (err) {
    console.error("Error during WebRTC setup:", err);
    alert("Failed to access camera and microphone. Please ensure they are connected and permissions are granted.");
  }
}

function startSendingVideoData(stream) {
  mediaRecorder = new MediaRecorder(stream, { mimeType: 'video/webm;codecs=vp8,opus' });

  mediaRecorder.ondataavailable = (event) => {
    if (event.data.size > 0) {
      recordedChunks.push(event.data);
      if (isVideoDataChannelOpen) {
        const reader = new FileReader();
        reader.onload = function () {
          if (videoDataChannel.readyState === 'open') {
            videoDataChannel.send(reader.result);
          }
        };
        reader.readAsArrayBuffer(event.data);
      }
    }
  };

  mediaRecorder.start(100); // Collect 100ms of data at a time
}

function startRecording() {
  if (!isWebSocketConnected) {
    console.warn("WebSocket is not connected.");
    return;
  }

  // Send a message to the server to start recording
  ws.send(JSON.stringify({ type: "start-recording" }));
  console.log("Started recording.");
  isRecording = true;
}

function stopRecording() {
  if (!isRecording) {
    console.warn("No recording in progress.");
    return;
  }

  // Send a message to the server to stop recording
  ws.send(JSON.stringify({ type: "stop-recording" }));
  console.log("Stopped recording.");

  isRecording = false;

  // Close the video data channel
  if (videoDataChannel) {
    videoDataChannel.close();
  }

  // Download the recorded video
  const blob = new Blob(recordedChunks, { type: 'video/webm' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.style.display = 'none';
  a.href = url;
  a.download = 'recording.webm';
  document.body.appendChild(a);
  a.click();
  window.URL.revokeObjectURL(url);
}

function cleanupConnection() {
  if (isRecording) {
    stopRecording();
  }
  if (peerConnection) {
    peerConnection.close();
  }
  if (localStream) {
    localStream.getTracks().forEach(track => track.stop());
  }
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.close();
  }
  if (mediaRecorder && mediaRecorder.state !== 'inactive') {
    mediaRecorder.stop();
  }
  isWebSocketConnected = false;
  isVideoDataChannelOpen = false;
  videoDataChannel = null;
  recordedChunks = [];
}
