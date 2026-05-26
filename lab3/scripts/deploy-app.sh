#!/usr/bin/env bash
# deploy-app.sh
# Runs ON the target node (via SSH from the runner).
# Pulls the new image from GHCR and restarts the systemd service.
#
# Required env vars (passed by the runner):
#   IMAGE_TAG  — the annotated tag that triggered the pipeline (e.g. v1.2.3)
set -euo pipefail

IMAGE="ghcr.io/fedorenkoivan/devops"
SERVICE="mywebapp-container"

: "${IMAGE_TAG:?IMAGE_TAG must be set}"

echo "Deploying image ${IMAGE}:${IMAGE_TAG} ..."

docker pull "${IMAGE}:${IMAGE_TAG}"
docker pull "${IMAGE}:stable"

sudo /usr/bin/systemctl restart "${SERVICE}"

echo "Deploy done. Waiting for service to become healthy..."
sleep 5
if sudo /usr/bin/systemctl is-active "${SERVICE}" > /dev/null 2>&1; then
  echo "Service is active."
else
  echo "Service failed to start!" >&2
  sudo /usr/bin/systemctl status "${SERVICE}" >&2
  exit 1
fi
