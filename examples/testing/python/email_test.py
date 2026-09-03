import json
import os
import smtplib
import time
import unittest
import urllib.parse
import urllib.request
import uuid
from email.message import EmailMessage


def delete_email(api_base, email_id):
    encoded_id = urllib.parse.quote(email_id, safe="")
    cleanup = urllib.request.Request(
        f"{api_base}/api/v1/emails/{encoded_id}", method="DELETE"
    )
    with urllib.request.urlopen(cleanup, timeout=5) as response:
        if response.status >= 300:
            raise RuntimeError(f"cleanup failed with HTTP status {response.status}")


def find_matching_email_ids(api_base, recipient, subject):
    query = urllib.parse.urlencode({"to": recipient, "limit": 10})
    with urllib.request.urlopen(
        f"{api_base}/api/v1/emails?{query}", timeout=5
    ) as response:
        if response.status >= 300:
            raise RuntimeError(f"list failed with HTTP status {response.status}")
        page = json.load(response)
    return [
        item["id"] for item in page["emails"] if item["subject"] == subject
    ]


def cleanup_captured_email(api_base, recipient, subject, state):
    email_ids = [state["id"]] if state["id"] else find_matching_email_ids(
        api_base, recipient, subject
    )
    if not email_ids:
        raise RuntimeError(
            f"cleanup could not locate the accepted message for {recipient}"
        )
    for email_id in email_ids:
        delete_email(api_base, email_id)


class OwlMailIntegrationTest(unittest.TestCase):
    def test_captured_email(self):
        smtp_host = os.getenv("TEST_SMTP_HOST", "127.0.0.1")
        smtp_port = int(os.getenv("TEST_SMTP_PORT", "1025"))
        api_base = os.getenv("TEST_MAIL_API", "http://127.0.0.1:1080").rstrip("/")
        run_id = uuid.uuid4().hex
        recipient = f"signup+{run_id}@example.test"
        subject = f"OwlMail integration {run_id}"
        token = f"token-{run_id}"

        message = EmailMessage()
        message["From"] = "sender@example.test"
        message["To"] = recipient
        message["Subject"] = subject
        message.set_content(f"Verification token: {token}")
        cleanup_state = {"id": None}
        with smtplib.SMTP(smtp_host, smtp_port, timeout=5) as client:
            client.send_message(message)
            self.addCleanup(
                cleanup_captured_email,
                api_base,
                recipient,
                subject,
                cleanup_state,
            )

        query = urllib.parse.urlencode({"to": recipient, "limit": 10})
        deadline = time.monotonic() + 15
        found = None
        while time.monotonic() < deadline:
            with urllib.request.urlopen(
                f"{api_base}/api/v1/emails?{query}", timeout=5
            ) as response:
                page = json.load(response)
            found = next(
                (item for item in page["emails"] if item["subject"] == subject), None
            )
            if found:
                break
            time.sleep(0.2)
        self.assertIsNotNone(found, f"timed out waiting for {recipient}")

        cleanup_state["id"] = found["id"]
        email_id = urllib.parse.quote(found["id"], safe="")
        with urllib.request.urlopen(
            f"{api_base}/api/v1/emails/{email_id}", timeout=5
        ) as response:
            detail = json.load(response)
        self.assertEqual(detail["subject"], subject)
        self.assertIn(token, detail["text"])


if __name__ == "__main__":
    unittest.main()
