const WebSocket = require('ws');
const http = require('http');
const express = require('express');

const app = express();
const server = http.createServer(app);
const wss = new WebSocket.Server({ server });

// Serve static files from the "public" directory (if needed)
app.use(express.static('public'));

wss.on('connection', (ws) => {
    console.log('A user connected');

    ws.on('message', (message) => {
        // Convert message from Buffer to string
        const messageStr = message.toString();
        console.log('Received message:', messageStr);

        // Parse the incoming message
        let parsedMessage;
        try {
            parsedMessage = JSON.parse(messageStr);
        } catch (e) {
            console.error('Failed to parse message:', e);
            return;
        }

        // Handle different message types
        switch (parsedMessage.type) {
            case 'offer':
            case 'answer':
            case 'candidate': // Ensure the candidate type matches client and Go script
                broadcastMessage(ws, parsedMessage);
                break;
            default:
                console.error('Unknown message type:', parsedMessage.type);
        }
    });

    ws.on('close', () => {
        console.log('A user disconnected');
    });
});

function broadcastMessage(sender, message) {
    wss.clients.forEach((client) => {
        if (client !== sender && client.readyState === WebSocket.OPEN) {
            client.send(JSON.stringify(message));
        }
    });
}

server.listen(3000, () => {
    console.log('Server is running on port 3000');
});





// const WebSocket = require('ws');
// const http = require('http');
// const express = require('express');

// const app = express();
// const server = http.createServer(app);
// const wss = new WebSocket.Server({ server });

// // Serve static files from the "public" directory (if needed)
// app.use(express.static('public'));

// wss.on('connection', (ws) => {
//     console.log('A user connected');

//     ws.on('message', (message) => {
//         console.log('Received message:', message);

//         // Parse the incoming message
//         let parsedMessage;
//         try {
//             parsedMessage = JSON.parse(message);
//         } catch (e) {
//             console.error('Failed to parse message:', e);
//             return;
//         }

//         // Handle different message types
//         switch (parsedMessage.type) {
//             case 'offer':
//             case 'answer':
//             case 'candidate': // Ensure the candidate type matches client and Go script
//                 broadcastMessage(ws, parsedMessage);
//                 break;
//             default:
//                 console.error('Unknown message type:', parsedMessage.type);
//         }
//     });

//     ws.on('close', () => {
//         console.log('A user disconnected');
//     });
// });

// function broadcastMessage(sender, message) {
//     wss.clients.forEach((client) => {
//         if (client !== sender && client.readyState === WebSocket.OPEN) {
//             client.send(JSON.stringify(message));
//         }
//     });
// }

// server.listen(3000, () => {
//     console.log('Server is running on port 3000');
// });