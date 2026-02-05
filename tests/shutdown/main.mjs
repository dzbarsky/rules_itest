import http from "http";
import fs from "fs";
import path from "path";

const WAIT_SECONDS = parseInt(process.env.SHUTDOWN_WAIT_SECONDS, 10) || 5;
const MARKER_DIR = process.env.MARKER_DIR || "";

function writeMarker(name) {
    if (MARKER_DIR) {
        const markerPath = path.join(MARKER_DIR, name);
        try {
            fs.writeFileSync(markerPath, name);
            console.log(`Wrote marker file: ${markerPath}`);
        } catch (err) {
            console.error(`Failed to write marker file: ${err}`);
        }
    }
}

const server = http.createServer(function (req, res) {
    res.writeHead(200);
    res.end("\n");
});
const port = process.argv[2];
server.listen(port, "127.0.0.1", () => {
    console.log("Server is running");
    writeMarker("server_started");
});

function shutdownHandler(signal) {
    console.log(
        `${signal} received. Waiting ${WAIT_SECONDS} seconds before shutting down...`,
    );
    writeMarker(`signal_${signal}`);
    setTimeout(() => {
        server.close(() => {
            console.log("Server closed.");
            writeMarker("shutdown_complete");
            process.exit(0);
        });
    }, WAIT_SECONDS * 1000);
}

process.on("SIGTERM", () => shutdownHandler("SIGTERM"));
