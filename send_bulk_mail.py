#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import argparse
import ipaddress
import random
import string
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from email.message import EmailMessage
import smtplib


def is_private_host(host: str) -> bool:
    try:
        ip = ipaddress.ip_address(host)
        return ip.is_private
    except ValueError:
        # If it's a hostname, we conservatively deny by default
        return False


def rand_text(n: int) -> str:
    alphabet = string.ascii_letters + string.digits + "     "
    return "".join(random.choice(alphabet) for _ in range(n)).strip() or "hello"


def build_message(i: int, from_addr: str, to_addr: str) -> EmailMessage:
    msg = EmailMessage()
    msg["From"] = from_addr
    msg["To"] = to_addr
    msg["Subject"] = f"owlmail load test #{i} {rand_text(10)}"
    # Make each message unique
    msg["Message-ID"] = f"<loadtest-{i}-{int(time.time()*1e6)}@local>"
    body = (
        f"Index: {i}\n"
        f"Time: {time.strftime('%Y-%m-%d %H:%M:%S')}\n"
        f"Payload: {rand_text(200)}\n"
    )
    msg.set_content(body)
    return msg


def send_one(host: str, port: int, timeout: int, i: int, from_addr: str, to_addr: str) -> None:
    msg = build_message(i, from_addr, to_addr)
    with smtplib.SMTP(host=host, port=port, timeout=timeout) as s:
        s.ehlo()
        s.send_message(msg)


def main():
    p = argparse.ArgumentParser(description="Send bulk test emails to a private SMTP server (no auth, no TLS).")
    p.add_argument("--host", default="192.168.123.200")
    p.add_argument("--port", type=int, default=1025)
    p.add_argument("--count", type=int, default=10000)
    p.add_argument("--concurrency", type=int, default=20)
    p.add_argument("--rate", type=float, default=200.0, help="Max emails per second (approx).")
    p.add_argument("--timeout", type=int, default=10)
    p.add_argument("--from", dest="from_addr", default="test@local")
    p.add_argument("--to", dest="to_addr", default="someone@example.com")
    p.add_argument("--allow-non-private", action="store_true", help="DANGEROUS: allow non-private targets.")
    args = p.parse_args()

    if not args.allow_non_private and not is_private_host(args.host):
        print(f"Refusing to send to non-private host: {args.host}\n"
              f"Use --allow-non-private only if you own/have permission.", file=sys.stderr)
        sys.exit(2)

    # Simple token-bucket-ish pacing
    min_interval = 1.0 / max(args.rate, 1e-9)
    last_send_at = 0.0

    sent = 0
    failed = 0
    start = time.time()

    def paced_send(i: int):
        nonlocal last_send_at
        # Pace globally (best-effort)
        while True:
            now = time.time()
            wait = (last_send_at + min_interval) - now
            if wait <= 0:
                last_send_at = now
                break
            time.sleep(min(wait, 0.05))

        send_one(args.host, args.port, args.timeout, i, args.from_addr, args.to_addr)

    print(f"Target SMTP: {args.host}:{args.port}")
    print(f"Count: {args.count}, Concurrency: {args.concurrency}, Rate: ~{args.rate}/s")
    print("Starting...")

    with ThreadPoolExecutor(max_workers=args.concurrency) as ex:
        futures = [ex.submit(paced_send, i) for i in range(1, args.count + 1)]
        for f in as_completed(futures):
            try:
                f.result()
                sent += 1
            except Exception as e:
                failed += 1
            total = sent + failed
            if total % 500 == 0:
                elapsed = time.time() - start
                qps = total / elapsed if elapsed > 0 else 0
                print(f"Progress {total}/{args.count} sent={sent} failed={failed} avg_rate={qps:.1f}/s")

    elapsed = time.time() - start
    print(f"Done. sent={sent} failed={failed} elapsed={elapsed:.1f}s avg_rate={(sent+failed)/elapsed:.1f}/s")


if __name__ == "__main__":
    main()

