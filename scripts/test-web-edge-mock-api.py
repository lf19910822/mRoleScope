#!/usr/bin/env python3
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    def log_message(self, _format, *_args):
        return

    def authorized(self):
        return self.headers.get("Cookie", "") == "mrolescope_access=valid-token"

    def send_json(self, status, payload):
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/api/v1/access/check":
            self.send_response(204 if self.authorized() else 401)
            self.end_headers()
            return
        if self.path == "/api/v1/jobs" and self.authorized():
            self.send_json(200, {"items": [{"id": "mock-job"}], "page": {"total": 1}})
            return
        if self.path == "/api/v1/jobs/mock-job" and self.authorized():
            self.send_json(200, {"id": "mock-job", "title": "mock"})
            return
        self.send_json(401, {"error": "unauthorized"})

    def do_POST(self):
        if self.path != "/api/v1/access/verify":
            self.send_json(404, {"error": "not_found"})
            return
        length = int(self.headers.get("Content-Length", "0"))
        payload = json.loads(self.rfile.read(length))
        if payload.get("invite_code") != "local-test-invite":
            self.send_json(401, {"error": "invalid_invite"})
            return
        self.send_response(204)
        self.send_header("Set-Cookie", "mrolescope_access=valid-token; Secure; HttpOnly; SameSite=Lax; Max-Age=259200; Path=/")
        self.end_headers()


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 18080
    server = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    server.serve_forever()
