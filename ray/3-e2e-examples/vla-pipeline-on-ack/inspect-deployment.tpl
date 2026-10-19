apiVersion: apps/v1
kind: Deployment
metadata:
  name: vla-artifact-inspector
  namespace: "${NAMESPACE}"
  labels:
    app.kubernetes.io/name: vla-artifact-inspector
    app.kubernetes.io/component: debug
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: vla-artifact-inspector
  template:
    metadata:
      labels:
        app.kubernetes.io/name: vla-artifact-inspector
        app.kubernetes.io/component: debug
    spec:
      imagePullSecrets:
        - name: "${IMAGE_PULL_SECRET_NAME}"
      containers:
        - name: inspector
          image: "${RAY_IMAGE}"
          imagePullPolicy: IfNotPresent
          workingDir: /data/oss/code
          command:
            - python
            - -c
            - "import time; time.sleep(10**9)"
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
            limits:
              cpu: "1"
              memory: 2Gi
          volumeMounts:
            - name: oss-root
              mountPath: /data/oss
              readOnly: true
            - name: oss-output
              mountPath: /data/output
      volumes:
        - name: oss-root
          persistentVolumeClaim:
            claimName: "${OSS_ROOT_PVC_NAME}"
            readOnly: true
        - name: oss-output
          persistentVolumeClaim:
            claimName: "${OSS_OUTPUT_PVC_NAME}"
