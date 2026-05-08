# Ausfallplan Notifier

Polls the school's [Ausfallplan page](https://stechlinsee-grundschule.de/ausfall-plan/) on a schedule, detects new entries, drops the ones that don't concern my children (3d, 6b), and pushes a [ntfy.sh](https://ntfy.sh) notification to my phone for the rest.

Design decisions, milestones, and status live in [PLAN.md](PLAN.md).

## Quick start (no notifications, no cloud)

```sh
cp .env.example .env
# edit .env: AUSFALL_URL=https://stechlinsee-grundschule.de/ausfall-plan/

go run ./cmd/local fetch     # parse and print the live page
go run ./cmd/local check     # diff against state.json, print additions
go run ./cmd/local check     # idempotent → "Keine neuen Einträge."
```

State persists to `./state.json` (gitignored). With `NTFY_TOPIC` unset, "notifications" go to stderr as `[notify dry-run] ...` lines.

## Configuration

`.env` (gitignored) — see `.env.example` for every available variable.

| Variable | Purpose |
| --- | --- |
| `AUSFALL_URL` | School page URL. Required. |
| `STATE_FILE` | File-store path. Default `state.json`. |
| `STATE_BACKEND` | `file` (default) or `dynamo`. |
| `CONFIG_FILE` | Blacklist config. Default `config.json`. |
| `NTFY_TOPIC` | ntfy.sh topic for push notifications. Unset → dry-run. |
| `NTFY_SERVER` | ntfy server. Default `https://ntfy.sh`. |
| `DDB_TABLE` | Dynamo table name. Default `ausfallplan-state`. |
| `DDB_ENDPOINT` | Override the AWS client BaseEndpoint (used for DynamoDB Local). |
| `AWS_REGION` | AWS region. |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | Dummy values are fine for DynamoDB Local. |

`config.json` holds the blacklist of class identifiers to drop. Anything ambiguous is kept (over-notify on doubt) — extend the blacklist incrementally as unwanted notifications arrive.

## Tests

```sh
go test ./...     # all unit tests, no Docker required
go vet ./...
```

The DynamoDB integration test (`./store -run Integration`) is gated by `DDB_TEST_ENDPOINT` and skips by default — see [DynamoDB Local](#dynamodb-local) below to run it.

## End-to-end with real ntfy.sh push

1. Pick an unguessable topic and put it in `.env`:
   ```sh
   echo "NTFY_TOPIC=ausfall-$(openssl rand -hex 8)" >> .env
   ```
   Topics on ntfy.sh are public; anyone who knows the name can read messages, hence the random suffix.

2. Subscribe on your phone — install the **ntfy** app (iOS / Android), tap **+**, paste the exact topic. Or open `https://ntfy.sh/<topic>` in a browser tab to test first.

3. Trigger a notification:
   ```sh
   rm -f state.json
   go run ./cmd/local check
   ```
   One push per surviving addition. Title = the Class field (e.g. `3d`, `6a,6b,6c`); body = `<weekday>, <date> · <hour> · <subject>`.

4. Switch back to dry-run by commenting `NTFY_TOPIC` out in `.env`.

### Forcing an error to test self-notify

Self-notify sends an urgent push (priority 5, tag `warning`) when a structural error occurs (parse or persist failure). To trigger it locally:

1. Set `SELF_NOTIFY=true` in `.env`.
2. Set `AUSFALL_URL` to a URL that returns HTTP 200 but is not the school page — for example `https://example.com`. The parser will fail because `tablepress-1` is absent.
3. Run:
   ```sh
   go run ./cmd/local check
   ```
   You should receive an urgent ntfy push titled `Ausfallplan-Notifier: Fehler in parse`.
4. Reset `AUSFALL_URL` to the real school URL and remove (or comment out) `SELF_NOTIFY=true` when done.

## DynamoDB Local

Runs the same Dynamo-backed pipeline that Lambda will use, without needing an AWS account.

1. Start DynamoDB Local:
   ```sh
   docker compose up -d
   ```
   The container runs in `-inMemory` mode — `docker compose down` wipes everything; recreate the table next time.

2. Create the table:
   ```sh
   AWS_ACCESS_KEY_ID=local AWS_SECRET_ACCESS_KEY=local AWS_REGION=us-east-1 \
     aws dynamodb create-table \
       --table-name ausfallplan-state \
       --attribute-definitions AttributeName=id,AttributeType=S \
       --key-schema AttributeName=id,KeyType=HASH \
       --billing-mode PAY_PER_REQUEST \
       --endpoint-url http://localhost:8000
   ```

3. Point the CLI at it via `.env`:
   ```
   STATE_BACKEND=dynamo
   DDB_ENDPOINT=http://localhost:8000
   AWS_REGION=us-east-1
   AWS_ACCESS_KEY_ID=local
   AWS_SECRET_ACCESS_KEY=local
   ```

4. Run:
   ```sh
   rm -f state.json     # so leftover file state doesn't confuse the diff
   go run ./cmd/local check
   go run ./cmd/local check     # idempotent
   ```

5. Inspect the stored item:
   ```sh
   AWS_ACCESS_KEY_ID=local AWS_SECRET_ACCESS_KEY=local AWS_REGION=us-east-1 \
     aws dynamodb scan \
       --table-name ausfallplan-state \
       --endpoint-url http://localhost:8000
   ```

6. Run the gated integration test:
   ```sh
   DDB_TEST_ENDPOINT=http://localhost:8000 \
     AWS_ACCESS_KEY_ID=local AWS_SECRET_ACCESS_KEY=local AWS_REGION=us-east-1 \
     go test -v ./store -run Integration
   ```

To go back to the file store, comment `STATE_BACKEND` out (or set it to `file`).

## Deploy to AWS

The Lambda function runs the same pipeline as `cmd/local check` but uses DynamoDB for state and reads all configuration from environment variables.

**Prerequisites**

- AWS CLI configured (`aws configure` or an IAM role/profile in `~/.aws/`).
- AWS SAM CLI installed ([installation guide](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html)).

**First deploy (guided)**

```sh
make build                        # cross-compiles arm64 Linux binary to build/bootstrap
sam deploy --guided               # walks through stack name, region, parameter values
```

When `--guided` prompts:
- **Stack name**: e.g. `ausfallplan-notifier`
- **AWS Region**: e.g. `eu-central-1`
- **AusfallURL**: accept the default or paste the school URL
- **NtfyTopic**: your unguessable ntfy.sh topic name (value is not echoed)
- **Blacklist**: accept the default to keep only `3d` and `6b` notifications
- Save the configuration to `samconfig.toml` when asked — subsequent deploys pick it up automatically.

**Subsequent deploys**

```sh
make deploy    # builds + sam deploy (reads samconfig.toml)
```

**Manual invocation**

```sh
make invoke    # fires one execution; output goes to stdout
```

Or with the AWS CLI directly:

```sh
aws lambda invoke --function-name ausfallplan-check /dev/stdout
```

The first invoke against an empty table will send notifications for all current entries; the second invoke sends nothing (idempotent).

> **Note:** Scheduling (EventBridge cron) is not yet wired — the function is invoke-on-demand only. Scheduling arrives in M8.
