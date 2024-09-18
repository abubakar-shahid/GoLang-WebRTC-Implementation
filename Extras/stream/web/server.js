const WebSocket = require('ws');
const server = new WebSocket.Server({ port: 8080 });

console.log('WebSocket server started at ws://localhost:8080');

const clients = new Set();

server.on('connection', (ws) => {
    clients.add(ws);

    ws.on('message', (message) => {
        console.log('Received message:', message);
        // Broadcast message to all clients except the sender
        clients.forEach(client => {
            if (client !== ws && client.readyState === WebSocket.OPEN) {
                client.send(message);
            }
        });
    });

    ws.on('close', () => {
        clients.delete(ws);
    });

    ws.on('error', (error) => {
        console.error('WebSocket Error:', error);
    });
});