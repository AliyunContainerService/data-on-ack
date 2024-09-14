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

export async function queryCronHistory(params) {
  const ret = await request(`${APIV1Prefix}/cron/history/${params.namespace}/${params.name}`, {
    params,
  });
  return {
    data: ret.data.cronHistories,
    total: ret.data.total,
  };
}
