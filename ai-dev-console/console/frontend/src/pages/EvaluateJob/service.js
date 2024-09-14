import request from "@/utils/request";

const APIV1Prefix = "/api/v1";

export async function queryJobs(params) {
    const ret = await request(`${APIV1Prefix}/evaluate/list`, {
        method: "GET",
        params,
    });
    return {
        data: ret.data.evaluateJobInfos,
        total: ret.data.total,
    };
}

export async function deleteJobs(namespace, name) {
    return request(
        `${APIV1Prefix}/evaluate/delete?namespace=${namespace}&name=${name}`,
        {
            method: "GET",
        }
        );
}