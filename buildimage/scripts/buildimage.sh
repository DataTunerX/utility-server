#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

log_with_timestamp() {
    local level=$1
    local message=$2
    echo "$(date '+%Y-%m-%d %H:%M:%S') [$level] $message"
}

error_handling() {
    local exit_code=$?
    log_with_timestamp "ERROR" "An error occurred. Exiting with status ${exit_code}"
    exit ${exit_code}
}

trap 'error_handling' ERR

log_with_timestamp "INFO" "Start download checkpoint file"
./s3downloader

log_with_timestamp "INFO" "Load image from tar file"
buildah pull docker-archive:ray271-llama2-7b-finetune.tar

log_with_timestamp "INFO" "Use $BASE_IMAGE image"
baseImage=$(buildah from $BASE_IMAGE)

log_with_timestamp "INFO" "Copy checkpoint file"
buildah copy $baseImage $MOUNT_PATH/$S3_FILEPATH /checkpoint/$S3_FILEPATH

log_with_timestamp "INFO" "chown -R ray:users /checkpoint/$S3_FILEPATH"
buildah run $baseImage -- sudo chown -R ray:users /checkpoint/$S3_FILEPATH

log_with_timestamp "INFO" "Commit image"
new_image=$(buildah commit $baseImage rayproject/$IMAGE_NAME:$IMAGE_TAG)

log_with_timestamp "INFO" "Logging in to the private registry"
buildah login --username $USERNAME --password $PASSWORD --tls-verify=false $REGISTRY_URL

log_with_timestamp "INFO" "Pushing image to repository"
buildah push --tls-verify=false $new_image $REGISTRY_URL/$REPOSITORY_NAME/$IMAGE_NAME:$IMAGE_TAG

log_with_timestamp "INFO" "Cleaning up downloaded files"
rm -rf $MOUNT_PATH/$S3_FILEPATH