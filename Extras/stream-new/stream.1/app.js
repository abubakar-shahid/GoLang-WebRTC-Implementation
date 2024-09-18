let mediaRecorder;
let recordedBlobs;

const codecPreferences = document.querySelector('#codecPreferences');
const errorMsgElement = document.querySelector('span#errorMsg');
const recordedVideo = document.querySelector('video#recorded');
const recordButton = document.querySelector('button#record');
const playButton = document.querySelector('button#play');
const downloadButton = document.querySelector('button#download');
let pc, sendChannel, receiveChannel;
const signaling = new WebSocket('ws://localhost:8080');

// Data channel elements
const startButton = document.getElementById('startButton');
const closeButton = document.getElementById('closeButton');
const sendButton = document.getElementById('sendButton');
const dataChannelSend = document.querySelector('textarea#dataChannelSend');
const dataChannelReceive = document.querySelector('textarea#dataChannelReceive');

signaling.onmessage = async (e) => {
  const data = JSON.parse(e.data);
  switch (data.type) {
    case 'offer':
      await handleOffer(data);
      break;
    case 'answer':
      await handleAnswer(data);
      break;
    case 'candidate':
      await handleCandidate(data);
      break;
    case 'ready':
      if (pc) return;
      startButton.disabled = false;
      break;
    case 'bye':
      if (pc) hangup();
      break;
  }
};

recordButton.addEventListener('click', () => {
  if (recordButton.textContent === 'Start Recording') {
    startRecording();
  } else {
    stopRecording();
    recordButton.textContent = 'Start Recording';
    playButton.disabled = false;
    downloadButton.disabled = false;
    codecPreferences.disabled = false;
  }
});

playButton.addEventListener('click', () => {
  const mimeType = codecPreferences.options[codecPreferences.selectedIndex].value.split(';', 1)[0];
  const superBuffer = new Blob(recordedBlobs, { type: mimeType });
  recordedVideo.src = window.URL.createObjectURL(superBuffer);
  recordedVideo.controls = true;
  recordedVideo.play();
});

downloadButton.addEventListener('click', () => {
  const mimeType = codecPreferences.options[codecPreferences.selectedIndex].value.split(';', 1)[0];
  const blob = new Blob(recordedBlobs, { type: mimeType });
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = mimeType === 'video/mp4' ? 'test.mp4' : 'test.webm';
  a.click();
});

async function startRecording() {
  recordedBlobs = [];
  const mimeType = codecPreferences.options[codecPreferences.selectedIndex].value;
  const options = { mimeType };
  try {
    mediaRecorder = new MediaRecorder(window.stream, options);
  } catch (e) {
    console.error('Exception while creating MediaRecorder:', e);
    errorMsgElement.innerHTML = `Error: ${JSON.stringify(e)}`;
    return;
  }
  recordButton.textContent = 'Stop Recording';
  playButton.disabled = true;
  downloadButton.disabled = true;
  codecPreferences.disabled = true;
  mediaRecorder.ondataavailable = handleDataAvailable;
  mediaRecorder.start();
}

function stopRecording() {
  mediaRecorder.stop();
}

function handleDataAvailable(event) {
  if (event.data && event.data.size > 0) {
    recordedBlobs.push(event.data);
  }
}

document.querySelector('button#start').addEventListener('click', async () => {
  document.querySelector('button#start').disabled = true;
  const constraints = {
    audio: { echoCancellation: true },
    video: { width: 1280, height: 720 }
  };
  try {
    const stream = await navigator.mediaDevices.getUserMedia(constraints);
    window.stream = stream;
    document.querySelector('video#gum').srcObject = stream;
    getSupportedMimeTypes().forEach(mimeType => {
      const option = document.createElement('option');
      option.value = mimeType;
      option.innerText = option.value;
      codecPreferences.appendChild(option);
    });
    codecPreferences.disabled = false;
  } catch (e) {
    console.error('Error accessing media devices.', e);
    errorMsgElement.innerHTML = `Error: ${e.toString()}`;
  }
});

function getSupportedMimeTypes() {
  const types = [
    'video/webm;codecs=vp9,opus',
    'video/mp4;codecs=h264,aac',
    'video/mp4'
  ];
  return types.filter(type => MediaRecorder.isTypeSupported(type));
}

// WebRTC Data Channel Logic
startButton.onclick = async () => {
  try {
    startButton.disabled = true;
    closeButton.disabled = false;
    await createPeerConnection();
    sendChannel = pc.createDataChannel('sendDataChannel');
    sendChannel.onopen = onSendChannelStateChange;
    sendChannel.onclose = onSendChannelStateChange;
    sendChannel.onmessage = onReceiveMessage;

    const offer = await pc.createOffer();
    signaling.send(JSON.stringify({ type: 'offer', sdp: offer.sdp }));
    await pc.setLocalDescription(offer);

    console.log('Connection is built');
  } catch (error) {
    console.error('Failed to build connection:', error);
    console.log('Connection lost');
  }
};

closeButton.onclick = () => {
  hangup();
  signaling.send(JSON.stringify({ type: 'bye' }));
  console.log('Connection is closed');
};

function createPeerConnection() {
  return new Promise((resolve, reject) => {
    pc = new RTCPeerConnection();
    pc.onicecandidate = event => {
      signaling.send(JSON.stringify({
        type: 'candidate',
        candidate: event.candidate ? event.candidate.candidate : null
      }));
    };
    pc.ondatachannel = event => {
      receiveChannel = event.channel;
      receiveChannel.onmessage = onReceiveMessage;
      receiveChannel.onopen = onReceiveChannelStateChange;
      receiveChannel.onclose = onReceiveChannelStateChange;
    };

    // Resolve promise once the peer connection is set up
    resolve();
  });
}

function onSendChannelStateChange() {
  if (sendChannel) {
    const readyState = sendChannel.readyState;
    console.log('Send channel state is: ' + readyState);
    if (readyState === 'open') {
      dataChannelSend.disabled = false;
      sendButton.disabled = false;
    } else {
      dataChannelSend.disabled = true;
      sendButton.disabled = true;
    }
  } else {
    dataChannelSend.disabled = true;
    sendButton.disabled = true;
  }
}

function onReceiveChannelStateChange() {
  if (receiveChannel) {
    const readyState = receiveChannel.readyState;
    console.log('Receive channel state is: ' + readyState);
    if (readyState === 'open') {
      dataChannelSend.disabled = false;
      sendButton.disabled = false;
    } else {
      dataChannelSend.disabled = true;
      sendButton.disabled = true;
    }
  } else {
    dataChannelSend.disabled = true;
    sendButton.disabled = true;
  }
}

function onReceiveMessage(event) {
  dataChannelReceive.value = event.data;
}

sendButton.onclick = () => {
  if (sendChannel) {
    if (recordedBlobs.length > 0) {
      // Send video blob
      const blob = new Blob(recordedBlobs, { type: 'video/webm' });
      sendChannel.send(blob);
      console.log('Sent video blob');
    } else {
      // Send text data if no video
      const data = dataChannelSend.value;
      sendChannel.send(data);
      console.log('Sent text data: ' + data);
    }
  }
};

function hangup() {
  if (pc) {
    pc.close();
    pc = null;
    console.log('Data channel is closed');
  }
  sendChannel = null;
  receiveChannel = null;
  startButton.disabled = false;
  sendButton.disabled = true;
  closeButton.disabled = true;
  dataChannelSend.value = '';
  dataChannelReceive.value = '';
}