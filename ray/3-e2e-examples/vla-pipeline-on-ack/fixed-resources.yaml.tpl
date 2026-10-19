apiVersion: v1
kind: Secret
metadata:
  name: "${IMAGE_PULL_SECRET_NAME}"
  namespace: "${NAMESPACE}"
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: "${DOCKER_CONFIG_JSON_BASE64}"
---
apiVersion: v1
kind: Secret
metadata:
  name: "${OSS_SECRET_NAME}"
  namespace: "${NAMESPACE}"
type: Opaque
data:
  akId: "${OSS_ACCESS_KEY_ID_BASE64}"
  akSecret: "${OSS_ACCESS_KEY_SECRET_BASE64}"
---
apiVersion: v1
kind: Secret
metadata:
  name: "${OPENAI_SECRET_NAME}"
  namespace: "${NAMESPACE}"
type: Opaque
data:
  OPENAI_API_KEY: "${OPENAI_API_KEY_BASE64}"
  OPENAI_BASE_URL: "${OPENAI_BASE_URL_BASE64}"
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: "${OSS_ROOT_PV_NAME}"
spec:
  capacity:
    storage: 100Gi
  accessModes:
    - ReadOnlyMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: ""
  volumeMode: Filesystem
  csi:
    driver: ossplugin.csi.alibabacloud.com
    volumeHandle: "${OSS_ROOT_PV_NAME}"
    nodePublishSecretRef:
      name: "${OSS_SECRET_NAME}"
      namespace: "${NAMESPACE}"
    volumeAttributes:
      bucket: "${OSS_ROOT_BUCKET}"
      path: "${OSS_ROOT_PATH}"
      url: "${OSS_ENDPOINT}"
      sigVersion: "${OSS_SIG_VERSION}"
      otherOpts: "${OSS_ROOT_OTHER_OPTS}"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: "${OSS_ROOT_PVC_NAME}"
  namespace: "${NAMESPACE}"
spec:
  accessModes:
    - ReadOnlyMany
  resources:
    requests:
      storage: 100Gi
  storageClassName: ""
  volumeMode: Filesystem
  volumeName: "${OSS_ROOT_PV_NAME}"
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: "${OSS_OUTPUT_PV_NAME}"
spec:
  capacity:
    storage: 500Gi
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: ""
  volumeMode: Filesystem
  csi:
    driver: ossplugin.csi.alibabacloud.com
    volumeHandle: "${OSS_OUTPUT_PV_NAME}"
    nodePublishSecretRef:
      name: "${OSS_SECRET_NAME}"
      namespace: "${NAMESPACE}"
    volumeAttributes:
      bucket: "${OSS_OUTPUT_BUCKET}"
      path: "${OSS_OUTPUT_PATH}"
      url: "${OSS_ENDPOINT}"
      sigVersion: "${OSS_SIG_VERSION}"
      otherOpts: "${OSS_OUTPUT_OTHER_OPTS}"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: "${OSS_OUTPUT_PVC_NAME}"
  namespace: "${NAMESPACE}"
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 500Gi
  storageClassName: ""
  volumeMode: Filesystem
  volumeName: "${OSS_OUTPUT_PV_NAME}"
