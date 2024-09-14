import request from "@/utils/request";

const APIV1Prefix = "/api/v1";

export async function listPVC(namespace) {
  return request(`${APIV1Prefix}/pvc/list`, {
    params: {
      namespace,
    },
  });
}

export async function listNotebook(namespaces, userName, userId) {
  return request(`${APIV1Prefix}/notebook/listFromStorage`, {
    params: {
      namespaces: JSON.stringify(namespaces),
      userName: userName,
      userId: userId,
    },
  });
}

export async function createNotebook(namespace, yaml) {
  return request(`${APIV1Prefix}/create`, {
    params: {
      namespace: namespace,
      template: yaml
    },
  });
}

export async function stopNotebook(namespace, name) {
  return request(`${APIV1Prefix}/notebook/stop`, {
    params: {
      namespace: namespace,
      name: name
    },
  });
}

export async function startNotebook(namespace, name) {
  return request(`${APIV1Prefix}/notebook/start`, {
    params: {
      namespace: namespace,
      name: name
    },
  });
}

export async function deleteNotebook(namespace, name) {
  return request(`${APIV1Prefix}/notebook/delete`, {
    params: {
      namespace: namespace,
      name: name,
    },
  });
}

export async function syncNotebooks(namespaces) {
  return request(`${APIV1Prefix}/notebook/sync`, {
    params: {
      namespaces: JSON.stringify(namespaces),
    },
  });
}