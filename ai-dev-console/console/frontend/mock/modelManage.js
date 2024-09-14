export default {
  "GET /mlflow-check-install": {
    code: "200",
    data: {
      install: true,
    },
  },
  "GET /api/v1/model/list": {
    code: "200",
    data: [
      {
        model_name: "model-0",
        model_latest_version: 1,
        model_created_by: "admin",
      },
      {
        model_name: "model-1",
        model_latest_version: 2,
        model_created_by: "researcher",
      },
    ],
  },
};
