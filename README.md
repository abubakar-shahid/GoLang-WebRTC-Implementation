# WebRTC Implementation Project

This project demonstrates a WebRTC implementation where a peer in JavaScript (on the web) communicates with a peer in Go. The system enables real-time video and audio sharing between the two peers via a data channel. The WebRTC connection is managed by a signaling server in Go, which facilitates the connection establishment and handles the closing process. Once the session ends, the Go client saves the shared audio and video.

## Table of Contents

- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Usage](#usage)

## Introduction

This project explores how WebRTC works by setting up peer-to-peer communication between a browser (web) peer and a server-side Go peer. The project illustrates the process of establishing a WebRTC connection using a signaling server implemented in Go. Video and audio streams are exchanged in real time, and the Go client captures and saves the media data once the connection is closed.

## Prerequisites

- Go (1.16 or higher)
- WebRTC-compatible browser (e.g., Chrome, Firefox)

## Installation

1. Clone this repository:

   ```bash
   git clone https://github.com/abubakar-shahid/GoLang-WebRTC-Implementation.git
   ```

2. Navigate to the project directory:

   ```bash
   cd GoLang-WebRTC-Implementation
   ```

3. Install Go dependencies:

   ```bash
   go mod tidy
   ```

4. Navigate to the Server Directory:

   ```bash
   cd engine/stream
   ```

5. Run the Go server:

   ```bash
   go run main.go
   ```

6. Open the web interface:
   
   Navigate to the web directory and open `index.html` in your browser. Simply double-click the `index.html` file or open it using your browser's "Open File" option.

## Usage

1. Start WebRTC Session Automatically:
   - Upon loading the web interface, the browser will prompt the user for permission to access their audio and video.
   - Once the user grants permissions, the WebRTC connection will automatically be initiated.
   - The Go signaling server will handle the request and establish a connection with the Go peer.
   - Real-time video and audio will be streamed from the web peer to the Go client.

2. Save Media:
   - When the WebRTC session ends, the Go client will automatically save the streamed audio and video.
