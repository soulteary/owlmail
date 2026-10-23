# Verification and delivery status in the mailbox

The Verify and Deliveries tabs are included in OwlMail **0.11.4**. They were not present in the original `v0.10.1` binary or image.

The existing HTML, plain text, headers, source, isolated preview, and preview width controls remain available. Two additional keyboard-accessible tabs help verify the selected message.

**Verify** checks expected subject text and message text with case-sensitive substring assertions. An optional comma-separated list of exact hostnames validates HTTP(S) links. The code extraction checkbox finds six-digit sequences; review the extracted values alongside individual checks. Empty expectations perform link inspection only. Truncation is explicitly shown and should not be treated as complete verification. Checks use the stored sanitized message and do not fetch URLs or mark messages read.

**Deliveries** manually refreshes the selected message's Relay and Webhook history. It shows failed outcomes, attempt counts, error categories, and persistence problems. A failure in one history source does not hide the other. Each list shows at most 100 records and indicates when more exist. Failed records have an explicit retry action with confirmation because it sends real outgoing requests. Accepted retries are shown as queued until the next refresh. Queued relays already recover automatically through the existing worker.

The test run identifier is also shown in message metadata when present. Verification inputs and results belong only to the selected message; switching messages discards them, and late responses cannot replace another message's view. New UI strings are English and use the existing locale fallback. No new framework or background polling is introduced.
