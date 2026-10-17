#!/bin/sh
# Supply a disposable DATABASE_URL for the API image; never source the local .env.
set -eu
component=${1:?Usage: smoke.sh api|frontend IMAGE}
image=${2:?Usage: smoke.sh api|frontend IMAGE}
case "$component" in
    api)
        : "${DATABASE_URL:?Use a disposable PostgreSQL instance}"
        container=$(docker run -d --network host \
            -e DATABASE_URL -e SERVER_PORT=18080 -e GIN_MODE=release \
            -e PAYOS_CLIENT_ID=ci-only -e PAYOS_API_KEY=ci-only -e PAYOS_CHECKSUM_KEY=ci-only \
            -e S3_ENDPOINT=http://127.0.0.1:1 -e S3_REGION=ci -e S3_BUCKET=rolecue-ci \
            -e S3_ACCESS_KEY_ID=ci-only -e S3_SECRET_ACCESS_KEY=ci-only \
            -e LLM_API_KEY=ci-only -e QUESTION_BANK_SIZE=0 \
            -e AVATAR_CONVERSION_ENABLED=true -e AVATAR_VRM_COMMERCIAL_USAGE=personalNonProfit \
            -e AVATAR_VRM_LICENSE_URL=https://example.test/license "$image")
        url=http://127.0.0.1:18080/health
        ;;
    frontend)
        container=$(docker run -d --network host -e PORT=13000 "$image")
        url=http://127.0.0.1:13000
        ;;
    *) echo 'Expected api or frontend' >&2; exit 1 ;;
esac
trap 'docker logs "$container"; docker rm -f "$container" >/dev/null' EXIT
attempt=0
while [ "$attempt" -lt 30 ]; do
    attempt=$((attempt + 1))
    if [ "$(docker inspect --format '{{.State.Running}}' "$container")" != true ]; then
        break
    fi
    if curl --fail --silent "$url" >/dev/null; then
        case "$(docker inspect --format '{{.Config.User}}' "$container")" in
            ''|0|0:*|root|root:*) echo 'Image must run as non-root' >&2; exit 1 ;;
        esac
        if [ "$component" = api ]; then
            docker exec "$container" pdftotext -v
            docker exec "$container" pdfinfo -v
        else
            asset=$(curl -fsS "$url" | python3 -c 'import re,sys; print(re.search(r"src=\"([^\"]+/_next/[^\"]+|/_next/[^\"]+)\"",sys.stdin.read()).group(1))')
            curl --fail --silent "$url$asset" >/dev/null
        fi
        exit 0
    fi
    sleep 1
done
echo "Image did not become healthy: $image" >&2
exit 1
