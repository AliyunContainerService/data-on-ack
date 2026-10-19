# Kubernetes namespace
export NAMESPACE="default"

# RayJob resource name
export RAY_JOB_NAME="vla-ray-job"

# Ray version; it must match the version in the container image.
export RAY_VERSION="2.56.0"

# Container image used by the Ray head and workers.
export RAY_IMAGE="<registry>/<repository>:<tag>"


# Name of the Kubernetes image pull Secret.
export IMAGE_PULL_SECRET_NAME="ray-image-pull-secret"

# Registry endpoint, for example registry-vpc.cn-hangzhou.aliyuncs.com.
export REGISTRY_SERVER="<registry-server>"

# Registry login username.
export REGISTRY_USERNAME="<registry-username>"

# Registry login password.
export REGISTRY_PASSWORD="<registry-password>"


# Name of the Kubernetes Secret that stores the OpenAI configuration.
export OPENAI_SECRET_NAME="ray-openai-secret"

# Raw OpenAI API key; the script encodes it with Base64.
export OPENAI_API_KEY="<openai-api-key>"

# Raw OpenAI-compatible endpoint; the script encodes it with Base64.
export OPENAI_BASE_URL="<openai-base-url>"


# Name of the Kubernetes Secret that stores OSS AccessKey credentials.
export OSS_SECRET_NAME="ray-oss-secret"

# OSS AccessKey ID
export OSS_ACCESS_KEY_ID="<oss-access-key-id>"

# OSS AccessKey Secret
export OSS_ACCESS_KEY_SECRET="<oss-access-key-secret>"

# OSS endpoint; use an internal endpoint in the cluster region.
export OSS_ENDPOINT="<oss-endpoint>"

# OSS signature version; ensure that the client supports V4.
export OSS_SIG_VERSION="v4"


# PV name for input data, models, and code.
export OSS_ROOT_PV_NAME="ray-vla-root-pv"

# PVC name for input data, models, and code.
export OSS_ROOT_PVC_NAME="ray-vla-root-pvc"

# OSS bucket for input data, models, and code.
export OSS_ROOT_BUCKET="<input-and-model-bucket>"

# Bucket prefix used as the mount root.
export OSS_ROOT_PATH="/<input-and-model-prefix>"


# PV name for output data.
export OSS_OUTPUT_PV_NAME="ray-vla-output-pv"

# PVC name for output data.
export OSS_OUTPUT_PVC_NAME="ray-vla-output-pvc"

# OSS bucket for job output.
export OSS_OUTPUT_BUCKET="<output-bucket>"

# Bucket prefix for job output.
export OSS_OUTPUT_PATH="/<output-prefix>"


required_vars=(
  NAMESPACE RAY_JOB_NAME RAY_VERSION RAY_IMAGE
  IMAGE_PULL_SECRET_NAME REGISTRY_SERVER REGISTRY_USERNAME REGISTRY_PASSWORD
  OPENAI_SECRET_NAME OPENAI_API_KEY OPENAI_BASE_URL
  OSS_SECRET_NAME OSS_ACCESS_KEY_ID OSS_ACCESS_KEY_SECRET OSS_ENDPOINT OSS_SIG_VERSION
  OSS_ROOT_PV_NAME OSS_ROOT_PVC_NAME OSS_ROOT_BUCKET OSS_ROOT_PATH
  OSS_OUTPUT_PV_NAME OSS_OUTPUT_PVC_NAME OSS_OUTPUT_BUCKET OSS_OUTPUT_PATH
)

for var_name in "${required_vars[@]}"; do
  var_value="${!var_name:-}"
  if [[ -z "${var_value}" || "${var_value}" == *"<"* || "${var_value}" == *">"* ]]; then
    echo "Variable ${var_name} is unset or still contains an example placeholder" >&2
    exit 1
  fi
done

export OSS_ACCESS_KEY_ID_BASE64="$(printf '%s' "${OSS_ACCESS_KEY_ID}" | base64 | tr -d '\n')"
export OSS_ACCESS_KEY_SECRET_BASE64="$(printf '%s' "${OSS_ACCESS_KEY_SECRET}" | base64 | tr -d '\n')"
export OPENAI_API_KEY_BASE64="$(printf '%s' "${OPENAI_API_KEY}" | base64 | tr -d '\n')"
export OPENAI_BASE_URL_BASE64="$(printf '%s' "${OPENAI_BASE_URL}" | base64 | tr -d '\n')"
REGISTRY_AUTH_BASE64="$(
  printf '%s' "${REGISTRY_USERNAME}:${REGISTRY_PASSWORD}" \
    | base64 \
    | tr -d '\r\n'
)"

export DOCKER_CONFIG_JSON_BASE64="$(
  printf '{"auths":{"%s":{"auth":"%s"}}}' \
    "${REGISTRY_SERVER}" \
    "${REGISTRY_AUTH_BASE64}" \
    | base64 \
    | tr -d '\r\n'
)"
