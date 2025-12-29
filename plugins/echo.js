// Echo JS Plugin - 回显插件示例
function metadata() {
    return {
        name: "echo-js",
        version: "1.0.0",
        type: "app",
        run_at: "client",
        description: "Echo plugin written in JavaScript",
        author: "GoTunnel"
    };
}

function start() {
    log("Echo JS plugin started");
}

function handleConn(conn) {
    log("New connection");
    while (true) {
        var data = conn.Read(4096);
        if (!data || data.length === 0) {
            break;
        }
        conn.Write(data);
    }
}

function stop() {
    log("Echo JS plugin stopped");
}
