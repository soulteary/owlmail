import io
import json
import types
import unittest
import urllib.error
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
    def test_delete_email_reports_non_success_status(self):
        with mock.patch(
            "examples.testing.python.email_test.urllib.request.urlopen",
            return_value=Response(status=500),
        ):
            with self.assertRaisesRegex(RuntimeError, "cleanup failed with HTTP status 500"):
                example.delete_email("http://owlmail.test", "captured/id")

    def test_delete_email_reports_transport_failure(self):
        failure = urllib.error.URLError("connection lost")
        with mock.patch(
            "examples.testing.python.email_test.urllib.request.urlopen",
            side_effect=failure,
        ):
            with self.assertRaises(urllib.error.URLError):
                example.delete_email("http://owlmail.test", "captured-id")

    def test_delete_email_does_not_accept_a_redirect(self):
        redirect = urllib.error.HTTPError(
            "http://owlmail.test/login", 302, "Found", {}, None
        )
        with mock.patch(
            "examples.testing.python.email_test.urllib.request.urlopen",
            side_effect=redirect,
        ):
            with self.assertRaises(urllib.error.HTTPError) as raised:
                example.delete_email("http://owlmail.test", "captured-id")
        self.assertEqual(raised.exception.code, 302)

    def test_cleanup_discovers_an_accepted_message_without_a_known_id(self):
        requests = []

        def urlopen(request, timeout):
            requests.append(request)
            if hasattr(request, "get_method") and request.get_method() == "DELETE":
                return Response(status=204)
            return Response(json.dumps({
                "emails": [
                    {"id": "other-id", "subject": "other"},
                    {"id": "captured/id", "subject": "subject"},
                ],
            }))

        with mock.patch(
            "examples.testing.python.email_test.urllib.request.urlopen",
            side_effect=urlopen,
        ):
            example.cleanup_captured_email(
                "http://owlmail.test",
                "recipient@example.test",
                "subject",
                {"id": None},
            )

        self.assertEqual(len(requests), 2)
        self.assertIn("to=recipient%40example.test", requests[0])
        self.assertEqual(requests[1].get_method(), "DELETE")
        self.assertTrue(requests[1].full_url.endswith("/captured%2Fid"))

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
