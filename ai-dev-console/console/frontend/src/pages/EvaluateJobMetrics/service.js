import request from "@/utils/request";

const APIV1Prefix = "/api/v1";

export async function getEvaluateJob(id) {
    return request(
        `${APIV1Prefix}/evaluate/get?id=${id}`,
        {
            method: "GET",
        }
        );
}