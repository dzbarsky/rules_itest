import http from "http";

const server = http.createServer(function (req, res) {
    res.writeHead(200);
    res.end("\n");
});
const port = process.argv[2];
server.listen(port, "127.0.0.1", () => {
    console.log('Server is running');
});