#!/usr/bin/env python3

import argparse
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlencode, urlparse


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--client-id", required=True)
    parser.add_argument("--client-secret", required=True)
    parser.add_argument("--code", default="smoke-google-code")
    parser.add_argument("--access-token", default="smoke-google-access-token")
    parser.add_argument("--subject", default="smoke-google-subject")
    parser.add_argument("--email", required=True)
    parser.add_argument("--name", default="Smoke User")
    return parser.parse_args()


def main():
    args = parse_args()

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            parsed = urlparse(self.path)
            if parsed.path == "/auth":
                self.handle_auth(parsed)
                return
            if parsed.path == "/userinfo":
                self.handle_userinfo()
                return
            self.send_error(404)

        def do_POST(self):
            parsed = urlparse(self.path)
            if parsed.path == "/token":
                self.handle_token()
                return
            self.send_error(404)

        def handle_auth(self, parsed):
            query = parse_qs(parsed.query)
            redirect_uri = query.get("redirect_uri", [""])[0].strip()
            state = query.get("state", [""])[0].strip()
            client_id = query.get("client_id", [""])[0].strip()
            response_type = query.get("response_type", [""])[0].strip()
            if client_id != args.client_id or response_type != "code" or redirect_uri == "" or state == "":
                self.send_error(400)
                return
            location = redirect_uri + ("&" if "?" in redirect_uri else "?") + urlencode({
                "code": args.code,
                "state": state,
            })
            self.send_response(302)
            self.send_header("Location", location)
            self.end_headers()

        def handle_token(self):
            try:
                length = int(self.headers.get("Content-Length", "0"))
            except ValueError:
                self.send_error(400)
                return
            form = parse_qs(self.rfile.read(length).decode("utf-8"))
            client_id = form.get("client_id", [""])[0].strip()
            client_secret = form.get("client_secret", [""])[0].strip()
            code = form.get("code", [""])[0].strip()
            grant_type = form.get("grant_type", [""])[0].strip()
            redirect_uri = form.get("redirect_uri", [""])[0].strip()
            if (
                client_id != args.client_id
                or client_secret != args.client_secret
                or code != args.code
                or grant_type != "authorization_code"
                or redirect_uri == ""
            ):
                self.send_error(401)
                return
            self.send_json({
                "access_token": args.access_token,
                "token_type": "Bearer",
                "id_token": "stub-id-token",
            })

        def handle_userinfo(self):
            auth = self.headers.get("Authorization", "").strip()
            if auth != f"Bearer {args.access_token}":
                self.send_error(401)
                return
            self.send_json({
                "sub": args.subject,
                "email": args.email,
                "email_verified": True,
                "name": args.name,
            })

        def send_json(self, payload):
            body = json.dumps(payload).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format, *args):
            return

    server = ThreadingHTTPServer((args.host, args.port), Handler)
    try:
        server.serve_forever()
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
