# Terraform Drift Detector

A cloud-agnostic Terraform drift detection platform that compares Terraform state files against live cloud infrastructure — without running `terraform plan` or `terraform apply`.

## Architecture

```
Terraform State → State Reader → Resource Extractor → Expected Model ─┐
                                                                     ├→ Drift Engine → Report Generator → CLI / JSON / Dashboard
Cloud APIs      → Cloud Fetcher → Resource Extractor → Actual Model ─┘
```

## Features

- **State ingestion** — Read local `terraform.tfstate` files
- **Cloud fetching** — AWS provider (EC2, S3, VPC, Subnet, Security Group, Lambda)
- **Drift detection** — Missing resources, attribute changes, tag changes
- **Multiple outputs** — CLI tables, JSON reports, web dashboard
- **Scheduling** — Cron-based recurring scans via `driftd`
- **REST API** — Trigger scans, query history, filter drifts
- **Extensible** — Plugin-based `CloudProvider` interface

## Quick Start

### Build

```bash
make build
```

### Run a scan (CLI)

```bash
# Requires AWS credentials in environment
./bin/driftctl scan --state testdata/sample.tfstate --provider aws --region us-east-1

# JSON output
./bin/driftctl scan --state testdata/sample.tfstate --provider aws --region us-east-1 --output json

# Using config file
./bin/driftctl scan -c configs/drift.example.yaml
```

### Start API server + dashboard

```bash
./bin/driftd -c configs/drift.example.yaml
# Open http://localhost:8080
```

## Configuration

See `configs/drift.example.yaml` for all options:

```yaml
state:
  source: local
  path: ./terraform.tfstate

providers:
  - name: aws
    regions: [us-east-1]
    credentials: env

schedules:
  - name: nightly
    cron: "0 2 * * *"
    enabled: true
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Health check |
| POST | `/api/v1/scans` | Trigger on-demand scan |
| GET | `/api/v1/scans` | List scan history |
| GET | `/api/v1/scans/{id}` | Get scan details |
| GET | `/api/v1/scans/{id}/report` | Full JSON drift report |
| GET | `/api/v1/scans/{id}/drifts?type=tags_changed` | Filtered drifts |
| POST | `/api/v1/schedules` | Create schedule |
| GET | `/api/v1/schedules` | List schedules |

## Drift Types

| Type | Description |
|------|-------------|
| `missing_in_cloud` | In Terraform state but not found in cloud |
| `missing_in_state` | In cloud but not managed by Terraform |
| `attribute_changed` | Non-tag attribute differs |
| `tags_changed` | Tag add/remove/update |
| `type_mismatch` | Resource type differs |

## Development

```bash
make test      # Run unit tests
make lint      # Run go vet
make run-scan  # Scan with sample state (cloud fetch may fail without creds)
```

## Project Structure

```
cmd/driftctl/     CLI tool
cmd/driftd/       API server + scheduler
internal/state/   Terraform state reader
internal/cloud/   Cloud provider plugins
internal/extract/ Resource normalization
internal/drift/   Comparison engine
internal/report/  Output formatters
internal/scan/    Scan orchestration
internal/api/     REST API
internal/store/   SQLite persistence
internal/schedule/ Cron scheduler
pkg/models/       Shared domain types
web/dashboard/    Web UI
testdata/         Test fixtures
```

## License

MIT
