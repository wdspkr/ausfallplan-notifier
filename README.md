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
