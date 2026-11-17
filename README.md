# Pixel Reviews Moderator

Asynchronous background worker service written in **Go** for content moderation of video game reviews in **Pixel Reviews**.

## Architecture

```mermaid
flowchart LR
    API[Pixel Reviews API] -->|Publishes message| EX[Exchange: moderation]
    EX --> Q["Quorum Queue (moderation.review.check)"]
    Q --> W[Go Moderator Worker]
    W --> J1[Judge 1: Deterministic Rules]
    J1 -->|Passes| J2[Judge 2: Gemini Decision API]
    J1 -->|Rejects| CB[Callback POST /internal/moderation/verdicts]
    J2 -->|Verdict| CB
    W -->|Failure after delivery limit x3| DLQ["DLX (moderation.dlx)"]
```

1. **Concurrent Message Consumption:** Consumes events from the RabbitMQ quorum queue `moderation.review.check`.
2. **Layered Evaluation Pipeline:**
   - **Judge 1 (Rules):** Deterministic check for spam, suspicious links, repetitive characters, leetspeak, and disallowed phrases. If rejected, external API quota is not consumed.
   - **Judge 2 (Gemini Decision API):** Context-aware machine learning evaluation for video game content using `gemini-decision-1`.
3. **Resilience & Dead Letter Exchange (DLX):** Configured with a delivery limit of 3 (`delivery-limit=3`). Unprocessable payloads and repeated network failures route to `moderation.dlx` to prevent poison message loops.
4. **Idempotent Verdict Submission:** Reports decisions back to the API via `POST /internal/moderation/verdicts` with service key authentication.

## Environment Variables

```env
MODERATION_SERVICE_KEY=supersecret_moderation_service_key_change_me_in_production
API_BASE_URL=http://localhost:8000
RABBITMQ_URL=amqp://pixel:pixel@localhost:5672
GEMINI_API_KEY=your_gemini_api_key_here
GEMINI_MODEL=gemini-decision-1
```

## Getting Started

### Local Development

```bash
go mod download
go test -v ./...
go run ./cmd/moderation
```

### Docker

```bash
docker build -t pixel-reviews-moderator .
docker run --rm --name pixel-moderator --env-file .env --network host pixel-reviews-moderator
```

### Docker Compose

This service is integrated into the root `docker-compose.yml` located in the `pixel-reviews-api` repository.
