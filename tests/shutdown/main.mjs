import http from "http";

const WAIT_SECONDS = parseInt(process.env.SHUTDOWN_WAIT_SECONDS, 10) || 5;

const server = http.createServer(function (req, res) {
    res.writeHead(200);
    res.end("\n");
});
const port = process.argv[2];
server.listen(port, "127.0.0.1", () => {
    console.log('Server is running');
});

function shutdownHandler(signal) {
    console.log(`${signal} received. Waiting ${WAIT_SECONDS} seconds before shutting down...`);
    setTimeout(() => {
        server.close(() => {
            console.log('Server closed.');
            process.exit(0);
        });
    }, WAIT_SECONDS * 1000);
}

process.on('SIGTERM', () => shutdownHandler('SIGTERM'));

