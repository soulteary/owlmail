# Go Report Card

The badge and detailed report are generated from repository source by the
[Go Report Card workflow](./workflows/go-reportcard.yml). The workflow runs
after Go source, `go.mod`, `go.sum`, or the workflow itself changes on `main`,
and it can also be started manually.

This page intentionally does not preserve file counts, issue line numbers, or
a copied grade between runs: those values become misleading as soon as source
moves. The generated `.github/goreportcard.svg` is the compact snapshot linked
from every language README. Use the workflow run and its committed report-card
artifacts as the result for the exact analyzed commit.

For required pull-request checks, rely on the race-enabled test and vet jobs in
[CI](./workflows/ci.yml), not on a previously generated report-card grade.
