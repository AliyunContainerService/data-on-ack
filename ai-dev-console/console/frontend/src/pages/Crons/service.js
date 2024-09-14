import request from "@/utils/request";

const APIV1Prefix = "/api/v1";

export async function queryCrons(params) {
  const ret = await request(`${APIV1Prefix}/cron/list`, {
    params,
  });
  return {
    data: ret.data.cronInfos,
    total: ret.data.total,
  };
}

export async function suspendCron(namespace, name, id) {
  return request(
      `${APIV1Prefix}/cron/suspend/${namespace}/${name}?id=${id}`,
      {
        method: "POST",
      }
  );
}

export async function resumeCron(namespace, name, id) {
  return request(
      `${APIV1Prefix}/cron/resume/${namespace}/${name}?id=${id}`,
      {
        method: "POST",
      }
  );
}

export async function deleteCron(namespace, name, id) {
  return request(
      `${APIV1Prefix}/cron/${namespace}/${name}?id=${id}`,
      {
        method: "DELETE",
      }
  );
}