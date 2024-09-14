# Notebook Controller

The controller allows users to create a custom resource "Notebook" (jupyter notebook).

## Build
You can build this controller by running the following command:
```shell
make build && make push
```

## Spec

The user needs to specify the PodSpec for the jupyter notebook.
For example:

```
apiVersion: kubeflow.org/v1alpha1
kind: Notebook
metadata:
  name: my-notebook
  namespace: test
spec:
  template:
    spec:  # Your PodSpec here
      containers:
      - image: gcr.io/kubeflow-images-public/tensorflow-1.10.1-notebook-cpu:v0.3.0
        args: ["start.sh", "lab", "--LabApp.token=''", "--LabApp.allow_remote_access='True'",
               "--LabApp.allow_root='True'", "--LabApp.ip='*'",
               "--LabApp.base_url=/test/my-notebook/",
               "--port=8888", "--no-browser"]
        name: notebook
      ...
```

The required fields are `containers[0].image` and (`containers[0].command` and/or `containers[0].args`).
That is, the user should specify what and how to run.

All other fields will be filled in with default value if not specified.

## Environment parameters
|Parameter | Description |
| --- | --- |
|ADD_FSGROUP| If the value is true or unset, fsGroup: 100 will be included in the pod's security context. If this value is present and set to false, it will suppress the automatic addition of fsGroup: 100 to the security context of the pod.|
|DEV| If the value is false or unset, then the default implementation of the Notebook Controller will be used. If the admins want to use a custom implementation from their local machine, they should set this value to true.|


   
## Commandline parameters

`metrics-addr`: The address the metric endpoint binds to. The default value is `:8080`.

`enable-leader-election`: Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager. The default value is `false`.
