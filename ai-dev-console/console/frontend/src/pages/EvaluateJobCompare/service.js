import request from "@/utils/request";

const APIV1Prefix = "/api/v1";

export async function getEvaluateJobCompareData(params) {
    return request(
        `${APIV1Prefix}/evaluate/compare`,
        {
            method: 'POST',
            body: params,
            headers: {
                'Content-Type': 'application/raw;charset=UTF-8',
            },
        }
        );
}