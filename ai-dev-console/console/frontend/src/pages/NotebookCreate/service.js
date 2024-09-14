import request from '@/utils/request';

const APIV1Prefix = '/api/v1';

// new code config
export async function newNotebookSource(params) {
  return request(`${APIV1Prefix}/notebook/create`, {
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
