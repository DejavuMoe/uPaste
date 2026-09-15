#!/usr/bin/env python3
"""Deterministic local Cap Standalone mock for browser qualification only.

It implements the small challenge/redeem/siteverify surface the pinned Cap
widget and uPaste verifier use. It is not shipped with uPaste and never
handles real secrets.
"""
import json
import sys
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 4193
SECRET = "test-secret"
TOKEN = "cap-test-token"


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        return

    def _json(self, payload, status=200):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Access-Control-Allow-Origin", "http://127.0.0.1:4176")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_OPTIONS(self):
        self.send_response(204)
        self.send_header("Access-Control-Allow-Origin", "http://127.0.0.1:4176")
        self.send_header("Access-Control-Allow-Methods", "POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")
        self.end_headers()

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        try:
            request = json.loads(raw.decode("utf-8"))
        except Exception:
            request = {}
        if self.path == "/challenge":
            self._json({
                "challenge": {"c": 1, "s": 4, "d": 2},
                "token": "mock-challenge-token",
                "expires": int(time.time() * 1000) + 60_000,
            })
        elif self.path == "/redeem":
            self._json({
                "success": True,
                "token": TOKEN,
                "expires": int(time.time() * 1000) + 60_000,
            })
        elif self.path == "/siteverify":
            success = request.get("secret") == SECRET and request.get("response") == TOKEN
            self._json({"success": success})
        else:
            self._json({"error": "not found"}, 404)


if __name__ == "__main__":
    HTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
