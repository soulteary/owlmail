# Go Report Card

**Grade: A+** (95.4%)

| Metric | Value |
| ------ | ----- |
| Files | 124 |
| Issues | 48 |

## Checks

| Check | Score |
| ----- | ----- |
| go_vet | 100% |
| gofmt | 100% |
| ineffassign | 100% |
| gocyclo | 61% |
| license | 100% |
| misspell | 100% |

## Issues

### gocyclo

- `internal/api/api_emails.go`
  - Line 473: warning: cyclomatic complexity 31 of function applyEmailFilters() is high (> 15) (gocyclo)
  - Line 350: warning: cyclomatic complexity 27 of function (*API).exportEmails() is high (> 15) (gocyclo)
  - Line 195: warning: cyclomatic complexity 16 of function parseEmailQuery() is high (> 15) (gocyclo)
- `internal/api/api_maildev_compat_test.go`
  - Line 63: warning: cyclomatic complexity 29 of function TestMailDevRESTFacadeContractAndReadSideEffect() is high (> 15) (gocyclo)
- `internal/api/api_test.go`
  - Line 331: warning: cyclomatic complexity 28 of function TestBasePathRouting() is high (> 15) (gocyclo)
- `internal/maildev/compat.go`
  - Line 416: warning: cyclomatic complexity 22 of function nestedValues() is high (> 15) (gocyclo)
- `internal/mailserver/read_only_test.go`
  - Line 470: warning: cyclomatic complexity 21 of function TestRefreshReadOnlyMailboxReappliesRepairedAttachmentMetadata() is high (> 15) (gocyclo)
  - Line 107: warning: cyclomatic complexity 16 of function TestRefreshReadOnlyMailboxDefersNewEventUntilEnvelopeMetadataIsReadable() is high (> 15) (gocyclo)
- `internal/config/file.go`
  - Line 20: warning: cyclomatic complexity 21 of function LoadConfigFile() is high (> 15) (gocyclo)
- `internal/api/api_emails_test.go`
  - Line 1073: warning: cyclomatic complexity 20 of function TestAPIGetEmailPreviewsWithFilters() is high (> 15) (gocyclo)
  - Line 2837: warning: cyclomatic complexity 19 of function TestApplyEmailFilters() is high (> 15) (gocyclo)
  - Line 692: warning: cyclomatic complexity 16 of function TestAPIGetAllEmailsWithFilters() is high (> 15) (gocyclo)
- `internal/mcpserver/server_test.go`
  - Line 25: warning: cyclomatic complexity 35 of function TestReadOnlyToolsUseMailboxSnapshots() is high (> 15) (gocyclo)
- `internal/config/config_test.go`
  - Line 364: warning: cyclomatic complexity 31 of function TestDefaultConfig() is high (> 15) (gocyclo)
  - Line 567: warning: cyclomatic complexity 29 of function TestDefineAndResolveConfig() is high (> 15) (gocyclo)
  - Line 512: warning: cyclomatic complexity 29 of function TestSMTPAndS3ConfigResolution() is high (> 15) (gocyclo)
- `internal/mailserver/mailserver_store_test.go`
  - Line 315: warning: cyclomatic complexity 18 of function TestMailServerDeleteAllEmail() is high (> 15) (gocyclo)
- `internal/mailserver/query_test.go`
  - Line 399: warning: cyclomatic complexity 16 of function legacyEmailMatches() is high (> 15) (gocyclo)
  - Line 65: warning: cyclomatic complexity 16 of function TestMailboxQueryReturnsDetachedResults() is high (> 15) (gocyclo)
- `internal/api/relay_jobs_test.go`
  - Line 131: warning: cyclomatic complexity 16 of function TestNativeRelayReturnsQueryableJob() is high (> 15) (gocyclo)
- `internal/mailserver/read_only.go`
  - Line 26: warning: cyclomatic complexity 62 of function (*MailServer).RefreshReadOnlyMailbox() is high (> 15) (gocyclo)
- `internal/mailserver/sqlite_index_test.go`
  - Line 60: warning: cyclomatic complexity 44 of function TestSQLiteMailboxIndexQueryAndRebuild() is high (> 15) (gocyclo)
  - Line 250: warning: cyclomatic complexity 19 of function TestMailServerUsesSQLiteIndexAndSynchronizesMutations() is high (> 15) (gocyclo)
  - Line 179: warning: cyclomatic complexity 17 of function TestSQLiteMailboxIndexPreservesWideMessageTimeRange() is high (> 15) (gocyclo)
- `internal/outgoing/smtp.go`
  - Line 141: warning: cyclomatic complexity 40 of function sendMailStreamWithConfig() is high (> 15) (gocyclo)
- `internal/api/api_mailcatcher_compat_test.go`
  - Line 35: warning: cyclomatic complexity 38 of function TestMailCatcherRESTFacadeContract() is high (> 15) (gocyclo)
- `internal/mailserver/attachment_migration_test.go`
  - Line 562: warning: cyclomatic complexity 24 of function TestAttachmentMigrationDryRunVerifiesRemoteAttachments() is high (> 15) (gocyclo)
  - Line 194: warning: cyclomatic complexity 19 of function TestAttachmentMigrationValidatesAttachmentFreeMessages() is high (> 15) (gocyclo)
  - Line 480: warning: cyclomatic complexity 16 of function TestAttachmentMigrationVerifiesRemoteOnlySourcesBeforeWrites() is high (> 15) (gocyclo)
- `internal/mailserver/attachment_storage.go`
  - Line 139: warning: cyclomatic complexity 23 of function (*MailServer).restoreLegacyLocalAttachmentMetadataContext() is high (> 15) (gocyclo)
- `internal/sendmail/sendmail_test.go`
  - Line 26: warning: cyclomatic complexity 21 of function TestParseArgs() is high (> 15) (gocyclo)
- `examples/testing/go/email_test.go`
  - Line 34: warning: cyclomatic complexity 19 of function TestCapturedEmail() is high (> 15) (gocyclo)
- `internal/config/validation.go`
  - Line 28: warning: cyclomatic complexity 81 of function ValidateConfig() is high (> 15) (gocyclo)
  - Line 249: warning: cyclomatic complexity 18 of function NormalizeBasePathname() is high (> 15) (gocyclo)
- `internal/api/api_config.go`
  - Line 107: warning: cyclomatic complexity 31 of function (*API).patchOutgoingConfig() is high (> 15) (gocyclo)
- `internal/outgoing/smtp_security_test.go`
  - Line 384: warning: cyclomatic complexity 24 of function startTestSMTPServer() is high (> 15) (gocyclo)
  - Line 240: warning: cyclomatic complexity 18 of function TestSendMailEnvelopeDeadlineCoversAllRecipients() is high (> 15) (gocyclo)
- `internal/api/relay_jobs.go`
  - Line 401: warning: cyclomatic complexity 23 of function (*API).enqueueRelayJob() is high (> 15) (gocyclo)
  - Line 331: warning: cyclomatic complexity 17 of function relayFailureCategory() is high (> 15) (gocyclo)
- `internal/webhook/service.go`
  - Line 265: warning: cyclomatic complexity 18 of function (*Service).runOutbox() is high (> 15) (gocyclo)
- `internal/mailserver/mailserver_utils_test.go`
  - Line 288: warning: cyclomatic complexity 17 of function TestSanitizeHTML() is high (> 15) (gocyclo)
- `internal/api/api_mcp_test.go`
  - Line 70: warning: cyclomatic complexity 16 of function TestAuthenticatedMCPProtocolUsesPrefixedRoute() is high (> 15) (gocyclo)
- `internal/mailserver/config.go`
  - Line 54: warning: cyclomatic complexity 33 of function NewMailServerWithOptions() is high (> 15) (gocyclo)
- `internal/api/api.go`
  - Line 190: warning: cyclomatic complexity 32 of function (*API).setupRoutes() is high (> 15) (gocyclo)
- `internal/mcpserver/workflows_test.go`
  - Line 18: warning: cyclomatic complexity 26 of function TestLatestAndEventDrivenWaitReturnBoundedSummaries() is high (> 15) (gocyclo)
  - Line 269: warning: cyclomatic complexity 25 of function TestReadOnlyResourcesAndPrompts() is high (> 15) (gocyclo)
- `internal/mailserver/mailserver_events_test.go`
  - Line 79: warning: cyclomatic complexity 17 of function TestOnWithConcurrencyBoundsHandlersBeforeStartingGoroutines() is high (> 15) (gocyclo)
- `internal/api/relay_jobs_persistence_test.go`
  - Line 253: warning: cyclomatic complexity 16 of function TestRetryRelayJobPreservesQueuedWorkDuringShutdown() is high (> 15) (gocyclo)
- `internal/sendmail/sendmail.go`
  - Line 168: warning: cyclomatic complexity 41 of function parseArgs() is high (> 15) (gocyclo)
  - Line 413: warning: cyclomatic complexity 26 of function prepareMessage() is high (> 15) (gocyclo)
  - Line 535: warning: cyclomatic complexity 17 of function submit() is high (> 15) (gocyclo)
- `internal/mailserver/store.go`
  - Line 896: warning: cyclomatic complexity 40 of function (*MailServer).LoadMailsFromDirectory() is high (> 15) (gocyclo)
  - Line 741: warning: cyclomatic complexity 32 of function (*MailServer).parseEmailMessage() is high (> 15) (gocyclo)
  - Line 384: warning: cyclomatic complexity 28 of function (*MailServer).DeleteAllEmail() is high (> 15) (gocyclo)
  - Line 46: warning: cyclomatic complexity 26 of function (*MailServer).saveEmailToStore() is high (> 15) (gocyclo)
- `cmd/owlmail/main.go`
  - Line 705: warning: cyclomatic complexity 33 of function createMailServer() is high (> 15) (gocyclo)
  - Line 501: warning: cyclomatic complexity 26 of function startAPIServer() is high (> 15) (gocyclo)
  - Line 905: warning: cyclomatic complexity 23 of function runMCPStdio() is high (> 15) (gocyclo)
  - Line 1038: warning: cyclomatic complexity 17 of function main() is high (> 15) (gocyclo)
- `internal/mailserver/query.go`
  - Line 301: warning: cyclomatic complexity 21 of function (compiledEmailQuery).matches() is high (> 15) (gocyclo)
  - Line 345: warning: cyclomatic complexity 17 of function sortEmailMatches() is high (> 15) (gocyclo)
- `internal/api/relay_jobs_persistence.go`
  - Line 62: warning: cyclomatic complexity 21 of function (*relayJobStore).load() is high (> 15) (gocyclo)
- `internal/mailserver/attachment_migration.go`
  - Line 240: warning: cyclomatic complexity 44 of function preflightAttachmentMigration() is high (> 15) (gocyclo)
  - Line 83: warning: cyclomatic complexity 41 of function MigrateLocalAttachments() is high (> 15) (gocyclo)
- `internal/config/validation_test.go`
  - Line 119: warning: cyclomatic complexity 37 of function TestValidateConfig() is high (> 15) (gocyclo)
- `internal/webhook/config.go`
  - Line 142: warning: cyclomatic complexity 32 of function compileTarget() is high (> 15) (gocyclo)
- `internal/mailserver/storage_transaction.go`
  - Line 30: warning: cyclomatic complexity 25 of function (*MailServer).storeIncomingEmail() is high (> 15) (gocyclo)
  - Line 292: warning: cyclomatic complexity 18 of function (*MailServer).recoverStorageArtifacts() is high (> 15) (gocyclo)
- `cmd/owlmail/main_test.go`
  - Line 79: warning: cyclomatic complexity 23 of function TestCompleteWebAuthConfig() is high (> 15) (gocyclo)
  - Line 927: warning: cyclomatic complexity 20 of function TestCreateMailServer() is high (> 15) (gocyclo)
  - Line 371: warning: cyclomatic complexity 18 of function TestSetupOutgoingConfig() is high (> 15) (gocyclo)
  - Line 220: warning: cyclomatic complexity 16 of function TestLoadAutoRelayRules() is high (> 15) (gocyclo)
- `internal/mcpserver/workflows.go`
  - Line 275: warning: cyclomatic complexity 18 of function (*Service).waitForEmail() is high (> 15) (gocyclo)
- `internal/attachmentstore/s3.go`
  - Line 70: warning: cyclomatic complexity 17 of function NewS3() is high (> 15) (gocyclo)
- `internal/webhook/payload.go`
  - Line 241: warning: cyclomatic complexity 23 of function matchTextChunk() is high (> 15) (gocyclo)
  - Line 174: warning: cyclomatic complexity 16 of function matchTextPattern() is high (> 15) (gocyclo)
- `internal/mcpserver/server.go`
  - Line 517: warning: cyclomatic complexity 20 of function makeEmailQuery() is high (> 15) (gocyclo)
- `internal/mailserver/attachment_storage_test.go`
  - Line 755: warning: cyclomatic complexity 19 of function TestS3ModeQuarantineRetriesRemoteCleanupAcrossRestart() is high (> 15) (gocyclo)
  - Line 168: warning: cyclomatic complexity 19 of function TestRemoteAttachmentDeleteFailureRecoversAcrossRestarts() is high (> 15) (gocyclo)
  - Line 105: warning: cyclomatic complexity 19 of function TestRemoteAttachmentLifecycleSurvivesRestart() is high (> 15) (gocyclo)
  - Line 601: warning: cyclomatic complexity 18 of function TestS3ModeRestoresSameSizeLegacyAttachmentsByContent() is high (> 15) (gocyclo)
  - Line 457: warning: cyclomatic complexity 18 of function TestS3ModeDeleteAllRetriesOnlyFailedEmails() is high (> 15) (gocyclo)
  - Line 656: warning: cyclomatic complexity 16 of function TestS3ModeDoesNotUpgradeAmbiguousLegacyAttachmentMetadata() is high (> 15) (gocyclo)
- `internal/api/api_mailcatcher_compat.go`
  - Line 73: warning: cyclomatic complexity 17 of function mailCatcherMessageDTO() is high (> 15) (gocyclo)

---

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action) on 2026-09-03 09:20:27 UTC._
