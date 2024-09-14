import request from '@/utils/request';

const APIV1Prefix = '/api/v1';

// new code config
export async function newEvaluateJobSource(params) {
  return request(`${APIV1Prefix}/evaluate/create`, {
    method: 'POST',
    body: JSON.stringify(params),
    headers: {
      'Content-Type': 'application/raw;charset=UTF-8',
    },
  });
}

export async function listPVC(namespace) {
  return request(`${APIV1Prefix}/notebook/listPVC`, {
    params: {
      namespace,
    },
  });
}

export async function getDatasources() {
  return request(`${APIV1Prefix}/datasource`);
}

export async function getCodeSource() {
  return request(`${APIV1Prefix}/codesource`);
}

export async function listImagePullSecrets() {
  return request(`${APIV1Prefix}/secret/image-pull-secrets?namespace=default`, {
    method: "GET",
  })
}