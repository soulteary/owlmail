import io
import json
import types
import unittest
from unittest import mock

from examples.testing.python import email_test as example


class Response(io.StringIO):
    def __init__(self, body="", status=200):
        super().__init__(body)
        self.status = status

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        self.close()


class SMTPClient:
    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        return None

    def send_message(self, message):
        return None


class CleanupTest(unittest.TestCase):
    def test_cleanup_runs_after_an_assertion_failure(self):
        requests = []

        def urlopen(request, timeout):
            requests.append(request)
            if hasattr(request, "get_method") and request.get_method() == "DELETE":
                return Response(status=204)
            if str(request).endswith("/api/v1/emails/captured-id"):
                return Response(json.dumps({"subject": "wrong subject", "text": ""}))
            return Response(json.dumps({
                "emails": [{"id": "captured-id", "subject": "OwlMail integration run"}],
            }))

        result = unittest.TestResult()
        case = example.OwlMailIntegrationTest("test_captured_email")
        with (
            mock.patch("examples.testing.python.email_test.smtplib.SMTP", return_value=SMTPClient()),
            mock.patch("examples.testing.python.email_test.urllib.request.urlopen", side_effect=urlopen),
            mock.patch("examples.testing.python.email_test.uuid.uuid4", return_value=types.SimpleNamespace(hex="run")),
        ):
            case.run(result)

        self.assertEqual(len(result.failures), 1)
        self.assertTrue(
            any(hasattr(request, "get_method") and request.get_method() == "DELETE" for request in requests),
            "cleanup DELETE was not attempted after the assertion failure",
        )


if __name__ == "__main__":
    unittest.main()
