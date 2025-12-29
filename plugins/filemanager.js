// FileManager JS Plugin - 文件管理插件
// 提供 HTTP API 管理客户端本地文件

var authToken = "";
var basePath = "/";

function metadata() {
    return {
        name: "filemanager",
        version: "1.0.0",
        type: "app",
        run_at: "client",
        description: "File manager with HTTP API",
        author: "GoTunnel"
    };
}

function start() {
    authToken = config("auth_token") || "admin";
    basePath = config("base_path") || "/";
    log("FileManager started, base: " + basePath);
}

function stop() {
    log("FileManager stopped");
}

// 处理连接
function handleConn(conn) {
    var data = conn.Read(4096);
    if (!data) return;

    var req = parseRequest(String.fromCharCode.apply(null, data));
    var resp = handleRequest(req);
    conn.Write(stringToBytes(resp));
}

// 解析 HTTP 请求
function parseRequest(raw) {
    var lines = raw.split("\r\n");
    var first = lines[0].split(" ");
    var req = {
        method: first[0] || "GET",
        path: first[1] || "/",
        headers: {},
        body: ""
    };

    var bodyStart = raw.indexOf("\r\n\r\n");
    if (bodyStart > 0) {
        req.body = raw.substring(bodyStart + 4);
    }
    return req;
}

// 处理请求
function handleRequest(req) {
    // 检查认证
    if (req.path.indexOf("?token=" + authToken) < 0) {
        return httpResponse(401, {error: "Unauthorized"});
    }

    var path = req.path.split("?")[0];

    if (path === "/api/list") {
        return handleList(req);
    } else if (path === "/api/read") {
        return handleRead(req);
    } else if (path === "/api/write") {
        return handleWrite(req);
    } else if (path === "/api/delete") {
        return handleDelete(req);
    }

    return httpResponse(404, {error: "Not found"});
}

// 获取查询参数
function getQueryParam(req, name) {
    var query = req.path.split("?")[1] || "";
    var params = query.split("&");
    for (var i = 0; i < params.length; i++) {
        var pair = params[i].split("=");
        if (pair[0] === name) {
            return decodeURIComponent(pair[1] || "");
        }
    }
    return "";
}

// 安全路径检查
function safePath(path) {
    if (!path) return basePath;
    // 防止路径遍历
    if (path.indexOf("..") >= 0) return null;
    if (path.charAt(0) !== "/") {
        path = basePath + "/" + path;
    }
    return path;
}

// 列出目录
function handleList(req) {
    var dir = safePath(getQueryParam(req, "path"));
    if (!dir) {
        return httpResponse(400, {error: "Invalid path"});
    }

    var entries = fs.readDir(dir);
    if (!entries) {
        return httpResponse(404, {error: "Directory not found"});
    }

    return httpResponse(200, {path: dir, entries: entries});
}

// 读取文件
function handleRead(req) {
    var file = safePath(getQueryParam(req, "path"));
    if (!file) {
        return httpResponse(400, {error: "Invalid path"});
    }

    var stat = fs.stat(file);
    if (!stat) {
        return httpResponse(404, {error: "File not found"});
    }
    if (stat.isDir) {
        return httpResponse(400, {error: "Cannot read directory"});
    }

    var content = fs.readFile(file);
    return httpResponse(200, {path: file, content: content, size: stat.size});
}

// 写入文件
function handleWrite(req) {
    var file = safePath(getQueryParam(req, "path"));
    if (!file) {
        return httpResponse(400, {error: "Invalid path"});
    }

    if (req.method !== "POST") {
        return httpResponse(405, {error: "Method not allowed"});
    }

    if (fs.writeFile(file, req.body)) {
        return httpResponse(200, {success: true, path: file});
    }
    return httpResponse(500, {error: "Write failed"});
}

// 删除文件
function handleDelete(req) {
    var file = safePath(getQueryParam(req, "path"));
    if (!file) {
        return httpResponse(400, {error: "Invalid path"});
    }

    if (!fs.exists(file)) {
        return httpResponse(404, {error: "File not found"});
    }

    if (fs.remove(file)) {
        return httpResponse(200, {success: true, path: file});
    }
    return httpResponse(500, {error: "Delete failed"});
}

// 构建 HTTP 响应
function httpResponse(status, data) {
    var body = JSON.stringify(data);
    var statusText = status === 200 ? "OK" :
                     status === 400 ? "Bad Request" :
                     status === 401 ? "Unauthorized" :
                     status === 404 ? "Not Found" :
                     status === 405 ? "Method Not Allowed" :
                     status === 500 ? "Internal Server Error" : "Unknown";

    return "HTTP/1.1 " + status + " " + statusText + "\r\n" +
           "Content-Type: application/json\r\n" +
           "Content-Length: " + body.length + "\r\n" +
           "Access-Control-Allow-Origin: *\r\n" +
           "\r\n" + body;
}

// 字符串转字节数组
function stringToBytes(str) {
    var bytes = [];
    for (var i = 0; i < str.length; i++) {
        bytes.push(str.charCodeAt(i));
    }
    return bytes;
}
