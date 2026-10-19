apiVersion: ray.io/v1
kind: RayJob
metadata:
  name: "${RAY_JOB_NAME}"
  namespace: "${NAMESPACE}"
  labels:
    app.kubernetes.io/name: vla-ray-job
    app.kubernetes.io/component: rayjob
spec:
  submissionMode: K8sJobMode
  entrypoint: "python /data/oss/code/vla_pipeline.py"
  shutdownAfterJobFinishes: true
  ttlSecondsAfterFinished: 600

  submitterPodTemplate:
    metadata:
      labels:
        app.kubernetes.io/name: vla-ray-job
        app.kubernetes.io/component: submitter
        app.kubernetes.io/instance: "${RAY_JOB_NAME}"
    spec:
      restartPolicy: Never
      imagePullSecrets:
        - name: "${IMAGE_PULL_SECRET_NAME}"
      containers:
        - name: ray-job-submitter
          image: "${RAY_IMAGE}"
          imagePullPolicy: IfNotPresent

  rayClusterSpec:
    rayVersion: "${RAY_VERSION}"
    enableInTreeAutoscaling: true
    autoscalerOptions:
      upscalingMode: Aggressive
      idleTimeoutSeconds: 120
      imagePullPolicy: IfNotPresent

    headGroupSpec:
      rayStartParams:
        dashboard-host: "0.0.0.0"
        num-cpus: "0"
        object-store-memory: "4294967296"
      template:
        metadata:
          labels:
            app.kubernetes.io/name: vla-ray-job
            app.kubernetes.io/component: head
            app.kubernetes.io/instance: "${RAY_JOB_NAME}"
        spec:
          imagePullSecrets:
            - name: "${IMAGE_PULL_SECRET_NAME}"
          containers:
            - name: ray-head
              image: "${RAY_IMAGE}"
              imagePullPolicy: IfNotPresent
              workingDir: /data/oss/code
              env:
                - name: PYTHONPATH
                  value: /data/oss/code
                - name: OPENAI_API_KEY
                  valueFrom:
                    secretKeyRef:
                      name: "${OPENAI_SECRET_NAME}"
                      key: OPENAI_API_KEY
                - name: OPENAI_BASE_URL
                  valueFrom:
                    secretKeyRef:
                      name: "${OPENAI_SECRET_NAME}"
                      key: OPENAI_BASE_URL
              ports:
                - name: gcs-server
                  containerPort: 6379
                  protocol: TCP
                - name: dashboard
                  containerPort: 8265
                  protocol: TCP
                - name: client
                  containerPort: 10001
                  protocol: TCP
              resources:
                requests:
                  cpu: "4"
                  memory: 16Gi
                limits:
                  cpu: "8"
                  memory: 24Gi
              volumeMounts:
                - name: oss-root
                  mountPath: /data/oss
                  readOnly: true
                - name: oss-output
                  mountPath: /data/output
                - name: dshm
                  mountPath: /dev/shm
                - name: raytmp
                  mountPath: /tmp/ray
                - name: workdir
                  mountPath: /data/work
          volumes:
            - name: oss-root
              persistentVolumeClaim:
                claimName: "${OSS_ROOT_PVC_NAME}"
                readOnly: true
            - name: oss-output
              persistentVolumeClaim:
                claimName: "${OSS_OUTPUT_PVC_NAME}"
            - name: dshm
              emptyDir:
                medium: Memory
                sizeLimit: 8Gi
            - name: raytmp
              emptyDir:
                sizeLimit: 30Gi
            - name: workdir
              persistentVolumeClaim:
                claimName: "${OSS_OUTPUT_PVC_NAME}"

    workerGroupSpecs:
      - groupName: gpu-worker
        replicas: 1
        minReplicas: 1
        maxReplicas: 1
        rayStartParams:
          num-cpus: "16"
          memory: "103079215104"
          object-store-memory: "25769803776"
        template:
          metadata:
            labels:
              app.kubernetes.io/name: vla-ray-job
              app.kubernetes.io/component: gpu-worker
              app.kubernetes.io/instance: "${RAY_JOB_NAME}"
          spec:
            imagePullSecrets:
              - name: "${IMAGE_PULL_SECRET_NAME}"
            tolerations:
              - key: nvidia.com/gpu
                operator: Exists
                effect: NoSchedule
            affinity:
              podAntiAffinity:
                requiredDuringSchedulingIgnoredDuringExecution:
                  - topologyKey: kubernetes.io/hostname
                    labelSelector:
                      matchLabels:
                        app.kubernetes.io/component: gpu-worker
                        app.kubernetes.io/instance: "${RAY_JOB_NAME}"
            containers:
              - name: ray-worker
                image: "${RAY_IMAGE}"
                imagePullPolicy: IfNotPresent
                workingDir: /data/oss/code
                env:
                  - name: PYTHONPATH
                    value: /data/oss/code
                  - name: OPENAI_API_KEY
                    valueFrom:
                      secretKeyRef:
                        name: "${OPENAI_SECRET_NAME}"
                        key: OPENAI_API_KEY
                  - name: OPENAI_BASE_URL
                    valueFrom:
                      secretKeyRef:
                        name: "${OPENAI_SECRET_NAME}"
                        key: OPENAI_BASE_URL
                resources:
                  requests:
                    cpu: "16"
                    memory: 96Gi
                    nvidia.com/gpu: "1"
                  limits:
                    cpu: "28"
                    memory: 160Gi
                    nvidia.com/gpu: "1"
                volumeMounts:
                  - name: oss-root
                    mountPath: /data/oss
                    readOnly: true
                  - name: oss-output
                    mountPath: /data/output
                  - name: dshm
                    mountPath: /dev/shm
                  - name: raytmp
                    mountPath: /tmp/ray
                  - name: workdir
                    mountPath: /data/work
            volumes:
              - name: oss-root
                persistentVolumeClaim:
                  claimName: "${OSS_ROOT_PVC_NAME}"
                  readOnly: true
              - name: oss-output
                persistentVolumeClaim:
                  claimName: "${OSS_OUTPUT_PVC_NAME}"
              - name: dshm
                emptyDir:
                  medium: Memory
                  sizeLimit: 32Gi
              - name: raytmp
                emptyDir:
                  sizeLimit: 100Gi
              - name: workdir
                persistentVolumeClaim:
                  claimName: "${OSS_OUTPUT_PVC_NAME}"

      - groupName: cpu-worker
        replicas: 0
        minReplicas: 0
        maxReplicas: 16
        rayStartParams:
          num-cpus: "16"
          memory: "68719476736"
          object-store-memory: "17179869184"
        template:
          metadata:
            labels:
              app.kubernetes.io/name: vla-ray-job
              app.kubernetes.io/component: cpu-worker
              app.kubernetes.io/instance: "${RAY_JOB_NAME}"
          spec:
            imagePullSecrets:
              - name: "${IMAGE_PULL_SECRET_NAME}"
            affinity:
              podAntiAffinity:
                requiredDuringSchedulingIgnoredDuringExecution:
                  - topologyKey: kubernetes.io/hostname
                    labelSelector:
                      matchLabels:
                        app.kubernetes.io/component: cpu-worker
                        app.kubernetes.io/instance: "${RAY_JOB_NAME}"
            containers:
              - name: ray-worker
                image: "${RAY_IMAGE}"
                imagePullPolicy: IfNotPresent
                workingDir: /data/oss/code
                env:
                  - name: PYTHONPATH
                    value: /data/oss/code
                  - name: OPENAI_API_KEY
                    valueFrom:
                      secretKeyRef:
                        name: "${OPENAI_SECRET_NAME}"
                        key: OPENAI_API_KEY
                  - name: OPENAI_BASE_URL
                    valueFrom:
                      secretKeyRef:
                        name: "${OPENAI_SECRET_NAME}"
                        key: OPENAI_BASE_URL
                resources:
                  requests:
                    cpu: "16"
                    memory: 72Gi
                  limits:
                    cpu: "24"
                    memory: 96Gi
                volumeMounts:
                  - name: oss-root
                    mountPath: /data/oss
                    readOnly: true
                  - name: oss-output
                    mountPath: /data/output
                  - name: dshm
                    mountPath: /dev/shm
                  - name: raytmp
                    mountPath: /tmp/ray
                  - name: workdir
                    mountPath: /data/work
            volumes:
              - name: oss-root
                persistentVolumeClaim:
                  claimName: "${OSS_ROOT_PVC_NAME}"
                  readOnly: true
              - name: oss-output
                persistentVolumeClaim:
                  claimName: "${OSS_OUTPUT_PVC_NAME}"
              - name: dshm
                emptyDir:
                  medium: Memory
                  sizeLimit: 24Gi
              - name: raytmp
                emptyDir:
                  sizeLimit: 60Gi
              - name: workdir
                persistentVolumeClaim:
                  claimName: "${OSS_OUTPUT_PVC_NAME}"
